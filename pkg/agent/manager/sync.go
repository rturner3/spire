package manager

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire/pkg/agent/client"
	"github.com/spiffe/spire/pkg/agent/manager/cache"
	"github.com/spiffe/spire/pkg/agent/workloadkey"
	"github.com/spiffe/spire/pkg/common/bundleutil"
	"github.com/spiffe/spire/pkg/common/telemetry"
	telemetry_agent "github.com/spiffe/spire/pkg/common/telemetry/agent"
	"github.com/spiffe/spire/pkg/common/util"
	"github.com/spiffe/spire/pkg/common/x509util"
	"github.com/spiffe/spire/proto/spire/common"
)

type csrRequest struct {
	EntryID              string
	SpiffeID             string
	CurrentSVIDExpiresAt time.Time
}

type SVIDCache interface {
	// UpdateEntries updates entries on cache
	UpdateEntries(update *cache.UpdateEntries, checkSVID func(*common.RegistrationEntry, *common.RegistrationEntry, *cache.X509SVID) bool)

	// UpdateSVIDs updates SVIDs on provided records
	UpdateSVIDs(update *cache.UpdateSVIDs)

	// GetStaleEntries gets a list of records that need update SVIDs
	GetStaleEntries() []*cache.StaleEntry

	// TaintX509SVIDs marks all SVIDs signed by a tainted X.509 authority as tainted
	// to force their rotation.
	TaintX509SVIDs(ctx context.Context, taintedX509Authorities []*x509.Certificate)

	// TaintJWTSVIDs removes JWT-SVIDs with tainted authorities from the cache,
	// forcing the server to issue a new JWT-SVID when one with a tainted
	// authority is requested.
	TaintJWTSVIDs(ctx context.Context, taintedJWTAuthorities map[string]struct{})
}

func (m *manager) syncSVIDs(ctx context.Context) (err error) {
	m.cache.SyncSVIDsWithSubscribers()
	return m.updateSVIDs(ctx, m.c.Log.WithField(telemetry.CacheType, "workload"), m.cache)
}

// processTaintedAuthorities verifies if a new authority is tainted and forces rotation in all caches if required.
func (m *manager) processTaintedAuthorities(ctx context.Context, bundle *spiffebundle.Bundle, x509Authorities []string, jwtAuthorities map[string]struct{}) error {
	newTaintedX509Authorities := getNewItemsFromSlice(m.processedTaintedX509Authorities, x509Authorities)
	if len(newTaintedX509Authorities) > 0 {
		m.c.Log.WithField(telemetry.SubjectKeyIDs, strings.Join(newTaintedX509Authorities, ",")).
			Debug("New tainted X.509 authorities found")

		taintedX509Authorities, err := bundleutil.FindX509Authorities(bundle, newTaintedX509Authorities)
		if err != nil {
			return fmt.Errorf("failed to search X.509 authorities: %w", err)
		}

		// Taint all regular X.509 SVIDs
		m.cache.TaintX509SVIDs(ctx, taintedX509Authorities)

		// Taint all SVIDStore SVIDs
		m.svidStoreCache.TaintX509SVIDs(ctx, taintedX509Authorities)

		// Notify rotator about new tainted authorities
		if err := m.svid.NotifyTaintedAuthorities(taintedX509Authorities); err != nil {
			return err
		}

		for _, subjectKeyID := range newTaintedX509Authorities {
			m.processedTaintedX509Authorities[subjectKeyID] = struct{}{}
		}
	}

	newTaintedJWTAuthorities := getNewItemsFromMap(m.processedTaintedJWTAuthorities, jwtAuthorities)
	if len(newTaintedJWTAuthorities) > 0 {
		m.c.Log.WithField(telemetry.JWTAuthorityKeyIDs, strings.Join(newTaintedJWTAuthorities, ",")).
			Debug("New tainted JWT authorities found")

		// Taint JWT-SVIDs in the cache
		m.cache.TaintJWTSVIDs(ctx, jwtAuthorities)

		for _, subjectKeyID := range newTaintedJWTAuthorities {
			m.processedTaintedJWTAuthorities[subjectKeyID] = struct{}{}
		}
	}

	return nil
}

// synchronize fetches the authorized entries from the server, updates the
// cache, and fetches missing/expiring SVIDs.
func (m *manager) synchronize(ctx context.Context) (err error) {
	cacheUpdate, storeUpdate, err := m.fetchEntries(ctx)
	if err != nil {
		return err
	}

	// Process all tainted authorities. The bundle is shared between both caches using regular cache data.
	if err := m.processTaintedAuthorities(ctx, cacheUpdate.Bundles[m.c.TrustDomain], cacheUpdate.TaintedX509Authorities, cacheUpdate.TaintedJWTAuthorities); err != nil {
		return err
	}

	if err := m.updateCache(ctx, cacheUpdate, m.c.Log.WithField(telemetry.CacheType, telemetry_agent.CacheTypeWorkload), "", m.cache); err != nil {
		return err
	}

	if err := m.updateCache(ctx, storeUpdate, m.c.Log.WithField(telemetry.CacheType, telemetry_agent.CacheTypeSVIDStore), telemetry_agent.CacheTypeSVIDStore, m.svidStoreCache); err != nil {
		return err
	}

	// Set last success sync
	m.setLastSync()
	return nil
}

func (m *manager) updateCache(ctx context.Context, update *cache.UpdateEntries, log logrus.FieldLogger, cacheType string, c SVIDCache) error {
	// update the cache and build a list of CSRs that need to be processed
	// in this interval.
	//
	// the values in `update` now belong to the cache. DO NOT MODIFY.
	var expiring int
	var outdated int
	c.UpdateEntries(update, func(existingEntry, newEntry *common.RegistrationEntry, svid *cache.X509SVID) bool {
		switch {
		case svid == nil:
			// no SVID
		case len(svid.Chain) == 0:
			// SVID has an empty chain. this is not expected to happen.
			log.WithFields(logrus.Fields{
				telemetry.RegistrationID: newEntry.EntryId,
				telemetry.SPIFFEID:       newEntry.SpiffeId,
			}).Warn("cached X509 SVID is empty")
		case m.c.RotationStrategy.ShouldRotateX509(m.c.Clk.Now(), svid.Chain[0]):
			expiring++
		case existingEntry != nil && existingEntry.RevisionNumber != newEntry.RevisionNumber:
			// Registration entry has been updated
			outdated++
		default:
			// SVID is good
			return false
		}

		return true
	})

	// TODO: this values are not real, we may remove
	if expiring > 0 {
		telemetry_agent.AddCacheManagerExpiredSVIDsSample(m.c.Metrics, cacheType, float32(expiring))
		log.WithField(telemetry.ExpiringSVIDs, expiring).Debug("Updating expiring SVIDs in cache")
	}
	if outdated > 0 {
		telemetry_agent.AddCacheManagerOutdatedSVIDsSample(m.c.Metrics, cacheType, float32(outdated))
		log.WithField(telemetry.OutdatedSVIDs, outdated).Debug("Updating SVIDs with outdated attributes in cache")
	}

	return m.updateSVIDs(ctx, log, c)
}

func (m *manager) updateSVIDs(ctx context.Context, log logrus.FieldLogger, c SVIDCache) error {
	m.updateSVIDMu.Lock()
	defer m.updateSVIDMu.Unlock()

	staleEntries := c.GetStaleEntries()
	if len(staleEntries) > 0 {
		var csrs []csrRequest
		sizeLimit := m.csrSizeLimitedBackoff.NextBackOff()
		log.WithFields(logrus.Fields{
			telemetry.Count: len(staleEntries),
			telemetry.Limit: sizeLimit,
		}).Debug("Renewing stale entries")

		for _, entry := range staleEntries {
			// we've exceeded the CSR limit, don't make any more CSRs
			if len(csrs) >= sizeLimit {
				break
			}

			csrs = append(csrs, csrRequest{
				EntryID:              entry.Entry.EntryId,
				SpiffeID:             entry.Entry.SpiffeId,
				CurrentSVIDExpiresAt: entry.SVIDExpiresAt,
			})
		}

		update, err := m.fetchSVIDs(ctx, csrs)
		if err != nil {
			return err
		}
		// the values in `update` now belong to the cache. DO NOT MODIFY.
		c.UpdateSVIDs(update)
	}
	return nil
}

func (m *manager) fetchSVIDs(ctx context.Context, csrs []csrRequest) (_ *cache.UpdateSVIDs, err error) {
	// Put all the CSRs in an array to make just one call with all the CSRs.
	counter := telemetry_agent.StartManagerFetchSVIDsUpdatesCall(m.c.Metrics)
	defer counter.Done(&err)
	defer func() {
		if err == nil {
			m.csrSizeLimitedBackoff.Success()
		}
	}()

	csrsIn := make(map[string][]byte)

	privateKeys := make(map[string]crypto.Signer, len(csrs))
	for _, csr := range csrs {
		log := m.c.Log.WithFields(logrus.Fields{
			"spiffe_id": csr.SpiffeID,
			"entry_id":  csr.EntryID,
		})
		if !csr.CurrentSVIDExpiresAt.IsZero() {
			log = log.WithField("expires_at", csr.CurrentSVIDExpiresAt.Format(time.RFC3339))
		}

		// Since entryIDs are unique, this shouldn't happen. Log just in case
		if _, ok := privateKeys[csr.EntryID]; ok {
			log.Warnf("Ignoring duplicate X509-SVID renewal for entry ID: %q", csr.EntryID)
			continue
		}

		if csr.CurrentSVIDExpiresAt.IsZero() {
			log.Info("Creating X509-SVID")
		} else {
			log.Info("Renewing X509-SVID")
		}

		spiffeID, err := spiffeid.FromString(csr.SpiffeID)
		if err != nil {
			return nil, err
		}
		privateKey, csrBytes, err := newCSR(spiffeID, m.c.WorkloadKeyType)
		if err != nil {
			return nil, err
		}
		privateKeys[csr.EntryID] = privateKey
		csrsIn[csr.EntryID] = csrBytes
	}

	svidsOut, err := m.client.NewX509SVIDs(ctx, csrsIn)
	if err != nil {
		// Reduce csr size for next invocation
		m.csrSizeLimitedBackoff.Failure()
		return nil, err
	}

	byEntryID := make(map[string]*cache.X509SVID, len(svidsOut))
	for entryID, svid := range svidsOut {
		privateKey, ok := privateKeys[entryID]
		if !ok {
			continue
		}
		chain, err := x509.ParseCertificates(svid.CertChain)
		if err != nil {
			return nil, err
		}

		svidLifetime := chain[0].NotAfter.Sub(chain[0].NotBefore)
		if m.c.RotationStrategy.ShouldFallbackX509DefaultRotation(svidLifetime) {
			log := m.c.Log.WithFields(logrus.Fields{
				"spiffe_id": chain[0].URIs[0].String(),
				"entry_id":  entryID,
			})
			log.Warn("X509 SVID lifetime isn't long enough to guarantee the availability_target, falling back to the default rotation strategy")
		}

		byEntryID[entryID] = &cache.X509SVID{
			Chain:      chain,
			PrivateKey: privateKey,
		}
	}

	return &cache.UpdateSVIDs{
		X509SVIDs: byEntryID,
	}, nil
}

// fetchEntries fetches entries that the agent is entitled to, divided in lists, one for regular entries and
// another one for storable entries
func (m *manager) fetchEntries(ctx context.Context) (_ *cache.UpdateEntries, _ *cache.UpdateEntries, err error) {
	// Put all the CSRs in an array to make just one call with all the CSRs.
	counter := telemetry_agent.StartManagerFetchEntriesUpdatesCall(m.c.Metrics)
	defer counter.Done(&err)

	var update *client.Update
	if m.c.UseSyncAuthorizedEntries {
		stats, err := m.client.SyncUpdates(ctx, m.syncedEntries, m.syncedBundles)
		if err != nil {
			return nil, nil, err
		}
		telemetry_agent.SetSyncStats(m.c.Metrics, stats)
		update = &client.Update{
			Entries: m.syncedEntries,
			Bundles: m.syncedBundles,
		}
	} else {
		update, err = m.client.FetchUpdates(ctx)
		if err != nil {
			return nil, nil, err
		}
	}

	bundles, err := parseBundles(update.Bundles)
	if err != nil {
		return nil, nil, err
	}

	// Get all Subject Key IDs and KeyIDs of tainted authorities
	var taintedX509Authorities []string
	taintedJWTAuthorities := make(map[string]struct{})
	if b, ok := update.Bundles[m.c.TrustDomain.IDString()]; ok {
		for _, rootCA := range b.RootCas {
			if rootCA.TaintedKey {
				cert, err := x509.ParseCertificate(rootCA.DerBytes)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to parse tainted x509 authority: %w", err)
				}
				subjectKeyID := x509util.SubjectKeyIDToString(cert.SubjectKeyId)
				taintedX509Authorities = append(taintedX509Authorities, subjectKeyID)
			}
		}
		for _, jwtKey := range b.JwtSigningKeys {
			if jwtKey.TaintedKey {
				taintedJWTAuthorities[jwtKey.Kid] = struct{}{}
			}
		}
	}

	cacheEntries := make(map[string]*common.RegistrationEntry)
	storeEntries := make(map[string]*common.RegistrationEntry)

	for entryID, entry := range update.Entries {
		switch {
		case entry.StoreSvid:
			storeEntries[entryID] = entry
		default:
			cacheEntries[entryID] = entry
		}
	}

	return &cache.UpdateEntries{
			Bundles:                bundles,
			RegistrationEntries:    cacheEntries,
			TaintedJWTAuthorities:  taintedJWTAuthorities,
			TaintedX509Authorities: taintedX509Authorities,
		}, &cache.UpdateEntries{
			Bundles:                bundles,
			RegistrationEntries:    storeEntries,
			TaintedJWTAuthorities:  taintedJWTAuthorities,
			TaintedX509Authorities: taintedX509Authorities,
		}, nil
}

func newCSR(spiffeID spiffeid.ID, keyType workloadkey.KeyType) (crypto.Signer, []byte, error) {
	pk, err := keyType.GenerateSigner()
	if err != nil {
		return nil, nil, err
	}

	csr, err := util.MakeCSR(pk, spiffeID)
	if err != nil {
		return nil, nil, err
	}
	return pk, csr, nil
}

func parseBundles(bundles map[string]*common.Bundle) (map[spiffeid.TrustDomain]*cache.Bundle, error) {
	out := make(map[spiffeid.TrustDomain]*cache.Bundle, len(bundles))
	for _, bundle := range bundles {
		bundle, err := bundleutil.SPIFFEBundleFromProto(bundle)
		if err != nil {
			return nil, err
		}
		td, err := spiffeid.TrustDomainFromString(bundle.TrustDomain().IDString())
		if err != nil {
			return nil, err
		}
		out[td] = bundle
	}
	return out, nil
}

func getNewItemsFromSlice(current map[string]struct{}, items []string) []string {
	var newItems []string
	for _, subjectKeyID := range items {
		if _, ok := current[subjectKeyID]; !ok {
			newItems = append(newItems, subjectKeyID)
		}
	}

	return newItems
}

func getNewItemsFromMap(current map[string]struct{}, items map[string]struct{}) []string {
	var newItems []string
	for subjectKeyID := range items {
		if _, ok := current[subjectKeyID]; !ok {
			newItems = append(newItems, subjectKeyID)
		}
	}

	return newItems
}
