package storage

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spiffe/spire/pkg/common/diskutil"
	"github.com/spiffe/spire/pkg/common/pemutil"
)

var (
	ErrNotCached = errors.New("not cached")
)

type Storage interface {
	// LoadSVID loads the SVID from storage. Returns ErrNotCached if the SVID
	// does not exist in the cache.
	LoadSVID() ([]*x509.Certificate, bool, error)

	// StoreSVID stores the SVID.
	StoreSVID(certs []*x509.Certificate, reattestable bool) error

	// DeleteSVID deletes the SVID.
	DeleteSVID() error

	// LoadBundle loads the bundle from storage. Returns ErrNotCached if the
	// bundle does not exist in the cache.
	LoadBundle() ([]*x509.Certificate, error)

	// StoreBundle stores the bundle.
	StoreBundle(certs []*x509.Certificate) error

	// LoadBootstrapState returns the Bootstrap state items
	LoadBootstrapState() (use int, start_time time.Time, connectionAttempts int, err error)

	// StoreBootstrapState stores the use and start_time bootstrap states for future use
	StoreBootstrapState(use int, start_time time.Time, connectionAttempts int) error

	// DeleteBootstrapState removes the bootstrap state
	DeleteBootstrapState() error
}

func Open(dir string) (Storage, error) {
	data, err := loadData(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return &storage{
		dir:  dir,
		data: data,
	}, nil
}

type storage struct {
	dir string

	mtx  sync.RWMutex
	data storageData
}

func (s *storage) LoadBundle() ([]*x509.Certificate, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if len(s.data.Bundle) == 0 {
		return nil, ErrNotCached
	}
	return s.data.Bundle, nil
}

func (s *storage) StoreBundle(bundle []*x509.Certificate) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	data := s.data
	data.Bundle = bundle

	if err := storeData(s.dir, data); err != nil {
		return err
	}

	s.data = data
	return nil
}

func (s *storage) LoadSVID() ([]*x509.Certificate, bool, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if len(s.data.SVID) == 0 {
		return nil, false, ErrNotCached
	}
	return s.data.SVID, s.data.Reattestable, nil
}

func (s *storage) StoreSVID(svid []*x509.Certificate, reattestable bool) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	data := s.data
	data.SVID = svid
	data.Reattestable = reattestable

	if err := storeData(s.dir, data); err != nil {
		return err
	}

	s.data = data
	return nil
}

func (s *storage) DeleteSVID() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	data := s.data
	data.SVID = nil
	data.Reattestable = false
	if err := storeData(s.dir, data); err != nil {
		return err
	}

	s.data = data
	return nil
}

func (s *storage) LoadBootstrapState() (use int, start_time time.Time, connectionAttempts int, err error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.data.BootstrapUse, s.data.BootstrapStartTime, s.data.ConnectionAttempts, nil
}
func (s *storage) StoreBootstrapState(use int, start_time time.Time, connectionAttempts int) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	data := s.data
	data.BootstrapUse = use
	data.BootstrapStartTime = start_time
	data.ConnectionAttempts = connectionAttempts
	if err := storeData(s.dir, data); err != nil {
		return err
	}

	s.data = data
	return nil
}

func (s *storage) DeleteBootstrapState() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	data := s.data
	data.BootstrapUse = 0
	data.BootstrapStartTime = time.Time{}
	data.ConnectionAttempts = 0
	if err := storeData(s.dir, data); err != nil {
		return err
	}

	s.data = data
	return nil
}

type storageJSON struct {
	SVID               [][]byte  `json:"svid"`
	Bundle             [][]byte  `json:"bundle"`
	Reattestable       bool      `json:"reattestable"`
	BootstrapUse       int       `json:"bootstrap_use"`
	BootstrapStartTime time.Time `json:"bootstrap_start_time"`
	ConnectionAttempts int       `json:"connection_attempts"`
}

type storageData struct {
	SVID               []*x509.Certificate
	Bundle             []*x509.Certificate
	Reattestable       bool
	BootstrapUse       int
	BootstrapStartTime time.Time
	ConnectionAttempts int
}

func (d storageData) MarshalJSON() ([]byte, error) {
	svid, err := encodeCertificates(d.SVID)
	if err != nil {
		return nil, fmt.Errorf("failed to encode SVID: %w", err)
	}
	bundle, err := encodeCertificates(d.Bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to encode bundle: %w", err)
	}
	return json.Marshal(storageJSON{
		SVID:               svid,
		Bundle:             bundle,
		Reattestable:       d.Reattestable,
		BootstrapUse:       d.BootstrapUse,
		BootstrapStartTime: d.BootstrapStartTime,
		ConnectionAttempts: d.ConnectionAttempts,
	})
}

func (d *storageData) UnmarshalJSON(b []byte) error {
	j := new(storageJSON)
	if err := json.Unmarshal(b, j); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}
	svid, err := parseCertificates(j.SVID)
	if err != nil {
		return fmt.Errorf("failed to parse SVID: %w", err)
	}
	bundle, err := parseCertificates(j.Bundle)
	if err != nil {
		return fmt.Errorf("failed to parse bundle: %w", err)
	}

	d.SVID = svid
	d.Bundle = bundle
	d.Reattestable = j.Reattestable
	d.BootstrapUse = j.BootstrapUse
	d.BootstrapStartTime = j.BootstrapStartTime
	d.ConnectionAttempts = j.ConnectionAttempts
	return nil
}

func storeData(dir string, data storageData) error {
	path := dataPath(dir)

	marshaled, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := diskutil.AtomicWritePrivateFile(path, marshaled); err != nil {
		return fmt.Errorf("failed to write data file: %w", err)
	}

	return nil
}

func loadData(dir string) (storageData, error) {
	path := dataPath(dir)

	marshaled, err := os.ReadFile(path)
	if err != nil {
		return storageData{}, fmt.Errorf("failed to read data: %w", err)
	}

	var data storageData
	if err := json.Unmarshal(marshaled, &data); err != nil {
		return storageData{}, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return data, nil
}

func parseCertificates(certsPEM [][]byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for _, certPEM := range certsPEM {
		cert, err := pemutil.ParseCertificate(certPEM)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func encodeCertificates(certs []*x509.Certificate) ([][]byte, error) {
	var certsPEM [][]byte
	for _, cert := range certs {
		if _, err := x509.ParseCertificate(cert.Raw); err != nil {
			return nil, err
		}
		certsPEM = append(certsPEM, pemutil.EncodeCertificate(cert))
	}
	return certsPEM, nil
}

func dataPath(dir string) string {
	return filepath.Join(dir, "agent-data.json")
}
