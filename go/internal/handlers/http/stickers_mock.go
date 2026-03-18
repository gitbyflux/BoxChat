package http

import (
	"errors"
	"mime/multipart"

	"gorm.io/gorm"
)

// DBInterface defines the database operations interface for testing
type DBInterface interface {
	Create(value interface{}) *gorm.DB
	First(dest interface{}, conds ...interface{}) *gorm.DB
	Find(dest interface{}, conds ...interface{}) *gorm.DB
	Where(query interface{}, args ...interface{}) *DBInterface
	Model(value interface{}) *DBInterface
	Delete(value interface{}, conds ...interface{}) *gorm.DB
	Preload(column string, conds ...interface{}) *DBInterface
	Updates(value interface{}) *gorm.DB
}

// FSInterface defines file system operations interface for testing
type FSInterface interface {
	MkdirAll(path string, perm uint32) error
	Stat(name string) (FileInfo, error)
	Remove(name string) error
}

// FileInfo is a subset of os.FileInfo for testing
type FileInfo interface {
	IsDir() bool
	ModTime() interface{}
	Mode() interface{}
	Name() string
	Size() int64
	Sys() interface{}
}

// RandInterface defines random number generation interface for testing
type RandInterface interface {
	Read(b []byte) (n int, err error)
}

// UploaderInterface defines file upload interface for testing
type UploaderInterface interface {
	SaveUploadedFile(file *multipart.FileHeader, dst string) error
}

// StickerDB wraps gorm.DB to implement DBInterface
type StickerDB struct {
	*gorm.DB
}

func (d *StickerDB) Create(value interface{}) *gorm.DB {
	return d.DB.Create(value)
}

func (d *StickerDB) First(dest interface{}, conds ...interface{}) *gorm.DB {
	return d.DB.First(dest, conds...)
}

func (d *StickerDB) Find(dest interface{}, conds ...interface{}) *gorm.DB {
	return d.DB.Find(dest, conds...)
}

func (d *StickerDB) Where(query interface{}, args ...interface{}) *DBInterface {
	var db DBInterface = d
	return &db
}

func (d *StickerDB) Model(value interface{}) *DBInterface {
	var db DBInterface = d
	return &db
}

func (d *StickerDB) Delete(value interface{}, conds ...interface{}) *gorm.DB {
	return d.DB.Delete(value, conds...)
}

func (d *StickerDB) Preload(column string, conds ...interface{}) *DBInterface {
	var db DBInterface = d
	return &db
}

func (d *StickerDB) Updates(value interface{}) *gorm.DB {
	return d.DB.Updates(value)
}

// MockDB implements DBInterface for testing
type MockDB struct {
	CreateFunc    func(value interface{}) *gorm.DB
	FirstFunc     func(dest interface{}, conds ...interface{}) *gorm.DB
	FindFunc      func(dest interface{}, conds ...interface{}) *gorm.DB
	WhereFunc     func(query interface{}, args ...interface{})
	ModelFunc     func(value interface{})
	DeleteFunc    func(value interface{}, conds ...interface{}) *gorm.DB
	PreloadFunc   func(column string, conds ...interface{})
	UpdatesFunc   func(value interface{}) *gorm.DB
	ShouldError   bool
	ErrorOnCall   string
	callCount     map[string]int
}

func NewMockDB() *MockDB {
	return &MockDB{
		callCount: make(map[string]int),
	}
}

func (m *MockDB) incrementCall(name string) {
	m.callCount[name]++
}

func (m *MockDB) Create(value interface{}) *gorm.DB {
	m.incrementCall("Create")
	if m.ShouldError && m.ErrorOnCall == "Create" {
		return &gorm.DB{Error: errors.New("mock database error")}
	}
	if m.CreateFunc != nil {
		return m.CreateFunc(value)
	}
	return &gorm.DB{}
}

func (m *MockDB) First(dest interface{}, conds ...interface{}) *gorm.DB {
	m.incrementCall("First")
	if m.ShouldError && m.ErrorOnCall == "First" {
		return &gorm.DB{Error: errors.New("mock database error")}
	}
	if m.FirstFunc != nil {
		return m.FirstFunc(dest, conds...)
	}
	return &gorm.DB{}
}

func (m *MockDB) Find(dest interface{}, conds ...interface{}) *gorm.DB {
	m.incrementCall("Find")
	if m.ShouldError && m.ErrorOnCall == "Find" {
		return &gorm.DB{Error: errors.New("mock database error")}
	}
	if m.FindFunc != nil {
		return m.FindFunc(dest, conds...)
	}
	return &gorm.DB{}
}

func (m *MockDB) Where(query interface{}, args ...interface{}) *DBInterface {
	m.incrementCall("Where")
	if m.WhereFunc != nil {
		m.WhereFunc(query, args...)
	}
	var db DBInterface = m
	return &db
}

func (m *MockDB) Model(value interface{}) *DBInterface {
	m.incrementCall("Model")
	if m.ShouldError && m.ErrorOnCall == "Model" {
		var db DBInterface = m
		return &db
	}
	if m.ModelFunc != nil {
		m.ModelFunc(value)
	}
	var db DBInterface = m
	return &db
}

func (m *MockDB) Delete(value interface{}, conds ...interface{}) *gorm.DB {
	m.incrementCall("Delete")
	if m.ShouldError && m.ErrorOnCall == "Delete" {
		return &gorm.DB{Error: errors.New("mock database error")}
	}
	if m.DeleteFunc != nil {
		return m.DeleteFunc(value, conds...)
	}
	return &gorm.DB{}
}

func (m *MockDB) Preload(column string, conds ...interface{}) *DBInterface {
	m.incrementCall("Preload")
	if m.PreloadFunc != nil {
		m.PreloadFunc(column, conds...)
	}
	var db DBInterface = m
	return &db
}

func (m *MockDB) Updates(value interface{}) *gorm.DB {
	m.incrementCall("Updates")
	if m.ShouldError && m.ErrorOnCall == "Updates" {
		return &gorm.DB{Error: errors.New("mock database error")}
	}
	if m.UpdatesFunc != nil {
		return m.UpdatesFunc(value)
	}
	return &gorm.DB{}
}

// MockFS implements FSInterface for testing
type MockFS struct {
	MkdirAllFunc func(path string, perm uint32) error
	StatFunc     func(name string) (FileInfo, error)
	RemoveFunc   func(name string) error
	ShouldError  bool
	ErrorOnCall  string
}

func (m *MockFS) MkdirAll(path string, perm uint32) error {
	if m.ShouldError && m.ErrorOnCall == "MkdirAll" {
		return errors.New("mock mkdir error")
	}
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	return nil
}

func (m *MockFS) Stat(name string) (FileInfo, error) {
	if m.ShouldError && m.ErrorOnCall == "Stat" {
		return nil, errors.New("mock stat error")
	}
	if m.StatFunc != nil {
		return m.StatFunc(name)
	}
	return nil, errors.New("file not found")
}

func (m *MockFS) Remove(name string) error {
	if m.ShouldError && m.ErrorOnCall == "Remove" {
		return errors.New("mock remove error")
	}
	if m.RemoveFunc != nil {
		return m.RemoveFunc(name)
	}
	return nil
}

// MockRand implements RandInterface for testing
type MockRand struct {
	ShouldError bool
	ReadFunc    func(b []byte) (n int, err error)
}

func (m *MockRand) Read(b []byte) (n int, err error) {
	if m.ShouldError {
		return 0, errors.New("mock rand error")
	}
	if m.ReadFunc != nil {
		return m.ReadFunc(b)
	}
	for i := range b {
		b[i] = 0
	}
	return len(b), nil
}

// MockFileInfo implements FileInfo for testing
type MockFileInfo struct {
	IsDirVal   bool
	NameVal    string
	SizeVal    int64
	ModTimeVal interface{}
	ModeVal    interface{}
	SysVal     interface{}
}

func (m *MockFileInfo) IsDir() bool           { return m.IsDirVal }
func (m *MockFileInfo) ModTime() interface{}  { return m.ModTimeVal }
func (m *MockFileInfo) Mode() interface{}     { return m.ModeVal }
func (m *MockFileInfo) Name() string          { return m.NameVal }
func (m *MockFileInfo) Size() int64           { return m.SizeVal }
func (m *MockFileInfo) Sys() interface{}      { return m.SysVal }
