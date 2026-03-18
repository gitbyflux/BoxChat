package http

import (
	"crypto/rand"
	"io"
	"os"

	"boxchat/internal/database"

	"gorm.io/gorm"
)

// Variables for mocking in tests
var (
	musicRandRead   = rand.Read
	musicFSMkdirAll = os.MkdirAll
	musicFSOpenFile = os.OpenFile
	musicFSRemove   = os.Remove
	musicFSStat     = os.Stat
)

// ============================================================================
// Database Interfaces
// ============================================================================

// MusicDB defines database operations for music handlers
type MusicDB interface {
	Create(value interface{}) *gorm.DB
	First(out interface{}, where ...interface{}) *gorm.DB
	Delete(value interface{}, where ...interface{}) *gorm.DB
	Where(query interface{}, args ...interface{}) *gorm.DB
	Order(order interface{}) *gorm.DB
	Find(out interface{}, where ...interface{}) *gorm.DB
	Error() error
}

// RealMusicDB wraps the global database.DB
type RealMusicDB struct {
	*gorm.DB
}

// NewRealMusicDB creates a new RealMusicDB instance
func NewRealMusicDB() MusicDB {
	return &RealMusicDB{DB: database.DB}
}

// Error returns the error from the underlying gorm.DB
func (r *RealMusicDB) Error() error {
	return r.DB.Error
}

// ============================================================================
// FileSystem Interfaces
// ============================================================================

// FileSystem defines file system operations
type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	Remove(name string) error
}

// RealFileSystem implements FileSystem using os package
type RealFileSystem struct{}

// NewRealFileSystem creates a new RealFileSystem instance
func NewRealFileSystem() FileSystem {
	return &RealFileSystem{}
}

// MkdirAll creates a directory and all necessary parents
func (fs *RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Stat returns file info
func (fs *RealFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// Remove removes a file
func (fs *RealFileSystem) Remove(name string) error {
	return os.Remove(name)
}

// ============================================================================
// Random Reader Interface
// ============================================================================

// RandomReader defines random bytes generation
type RandomReader interface {
	Read(p []byte) (n int, err error)
}

// RealRandomReader implements RandomReader using crypto/rand
type RealRandomReader struct{}

// NewRealRandomReader creates a new RealRandomReader instance
func NewRealRandomReader() RandomReader {
	return &RealRandomReader{}
}

// Read generates random bytes
func (r *RealRandomReader) Read(p []byte) (int, error) {
	return rand.Read(p)
}

// ============================================================================
// Mock Implementations for Testing
// ============================================================================

// MockMusicDB is a mock implementation of MusicDB for testing
type MockMusicDB struct {
	CreateFunc    func(value interface{}) *gorm.DB
	FirstFunc     func(out interface{}, where ...interface{}) *gorm.DB
	DeleteFunc    func(value interface{}, where ...interface{}) *gorm.DB
	WhereFunc     func(query interface{}, args ...interface{}) *gorm.DB
	OrderFunc     func(order interface{}) *gorm.DB
	FindFunc      func(out interface{}, where ...interface{}) *gorm.DB
	ErrorValue    error
	LastQueryType string
}

// NewMockMusicDB creates a new MockMusicDB with default implementations
func NewMockMusicDB() *MockMusicDB {
	return &MockMusicDB{
		CreateFunc: func(value interface{}) *gorm.DB {
			return &gorm.DB{Error: nil}
		},
		FirstFunc: func(out interface{}, where ...interface{}) *gorm.DB {
			return &gorm.DB{Error: nil}
		},
		DeleteFunc: func(value interface{}, where ...interface{}) *gorm.DB {
			return &gorm.DB{Error: nil}
		},
		WhereFunc: func(query interface{}, args ...interface{}) *gorm.DB {
			return &gorm.DB{Error: nil}
		},
		OrderFunc: func(order interface{}) *gorm.DB {
			return &gorm.DB{Error: nil}
		},
		FindFunc: func(out interface{}, where ...interface{}) *gorm.DB {
			return &gorm.DB{Error: nil}
		},
	}
}

// Create mocks the Create operation
func (m *MockMusicDB) Create(value interface{}) *gorm.DB {
	m.LastQueryType = "Create"
	return m.CreateFunc(value)
}

// First mocks the First operation
func (m *MockMusicDB) First(out interface{}, where ...interface{}) *gorm.DB {
	m.LastQueryType = "First"
	return m.FirstFunc(out, where...)
}

// Delete mocks the Delete operation
func (m *MockMusicDB) Delete(value interface{}, where ...interface{}) *gorm.DB {
	m.LastQueryType = "Delete"
	return m.DeleteFunc(value, where...)
}

// Where mocks the Where operation
func (m *MockMusicDB) Where(query interface{}, args ...interface{}) *gorm.DB {
	m.LastQueryType = "Where"
	return m.WhereFunc(query, args...)
}

// Order mocks the Order operation
func (m *MockMusicDB) Order(order interface{}) *gorm.DB {
	m.LastQueryType = "Order"
	return m.OrderFunc(order)
}

// Find mocks the Find operation
func (m *MockMusicDB) Find(out interface{}, where ...interface{}) *gorm.DB {
	m.LastQueryType = "Find"
	return m.FindFunc(out, where...)
}

// Error returns the mock error value
func (m *MockMusicDB) Error() error {
	return m.ErrorValue
}

// MockFileSystem is a mock implementation of FileSystem for testing
type MockFileSystem struct {
	MkdirAllFunc func(path string, perm os.FileMode) error
	StatFunc     func(name string) (os.FileInfo, error)
	RemoveFunc   func(name string) error
}

// NewMockFileSystem creates a new MockFileSystem with default implementations
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		MkdirAllFunc: func(path string, perm os.FileMode) error {
			return nil
		},
		StatFunc: func(name string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		RemoveFunc: func(name string) error {
			return nil
		},
	}
}

// MkdirAll mocks the MkdirAll operation
func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return m.MkdirAllFunc(path, perm)
}

// Stat mocks the Stat operation
func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	return m.StatFunc(name)
}

// Remove mocks the Remove operation
func (m *MockFileSystem) Remove(name string) error {
	return m.RemoveFunc(name)
}

// MockRandomReader is a mock implementation of RandomReader for testing
type MockRandomReader struct {
	ReadFunc func(p []byte) (n int, err error)
}

// NewMockRandomReader creates a new MockRandomReader
func NewMockRandomReader() *MockRandomReader {
	return &MockRandomReader{
		ReadFunc: func(p []byte) (int, error) {
			for i := range p {
				p[i] = 0
			}
			return len(p), nil
		},
	}
}

// Read mocks the Read operation
func (m *MockRandomReader) Read(p []byte) (int, error) {
	return m.ReadFunc(p)
}

// NewMockRandomReaderWithError returns a reader that always returns error
func NewMockRandomReaderWithError() *MockRandomReader {
	return &MockRandomReader{
		ReadFunc: func(p []byte) (int, error) {
			return 0, io.ErrUnexpectedEOF
		},
	}
}
