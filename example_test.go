package gtest_test

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/houqp/gtest"
	"github.com/stretchr/testify/assert"
)

// start of fixture definitions

type WorkDirFixture struct{}

// Construct can take other fixtures as input parameter as well
func (s WorkDirFixture) Construct(t *testing.T, fixtures struct{}) (string, string) {
	dir, err := ioutil.TempDir("", "gtest-fixture")
	assert.NoError(t, err)

	// First return value will be passed to test as fixture value, second return value
	// will be passed to Destruct as ctx for cleanup purpose if needed.
	return dir, dir
}

func (s WorkDirFixture) Destruct(t *testing.T, dir string) {
	os.RemoveAll(dir)
}

type UidFixture struct {
	CurrentId int
}

func (s *UidFixture) Construct(t *testing.T, fixtures struct{}) (int, interface{}) {
	s.CurrentId += 1
	// you can return anything in second value if you don't intend to
	// Destruct fixture value
	return s.CurrentId, nil
}

func (s *UidFixture) Destruct(t *testing.T, ctx interface{}) {}

func init() {
	// each fixture needs to be registered to be accessible from tests
	// ScopeCall fixture will return different value for each reference in fixtures struct
	gtest.MustRegisterFixture("Uid", &UidFixture{}, gtest.ScopeCall)
	// ScopeSubTest fixture's value will be cached, so multiple reference in
	// the same subtest will result in the same cached value.
	gtest.MustRegisterFixture("WorkDir", &WorkDirFixture{}, gtest.ScopeSubTest)
}

// start of test definitions

type SampleTests struct{}

func (s *SampleTests) Setup(t *testing.T)      {}
func (s *SampleTests) Teardown(t *testing.T)   {}
func (s *SampleTests) BeforeEach(t *testing.T) {}
func (s *SampleTests) AfterEach(t *testing.T)  {}

func (s *SampleTests) SubTestCompare(t *testing.T) {
	if 1 != 1 {
		t.FailNow()
	}
}

func (s *SampleTests) SubTestCheckPrefix(t *testing.T) {
	if !strings.HasPrefix("abc", "ab") {
		t.FailNow()
	}
}

func (s *SampleTests) SubTestMultipleFixtures(t *testing.T, fixtures struct {
	DirPath string `fixture:"WorkDir"`
	Uid1    int    `fixture:"Uid"`
	Uid2    int    `fixture:"Uid"`
}) {
	info, err := os.Stat(fixtures.DirPath)
	if err != nil {
		t.FailNow()
		return
	}

	// Uid fixture has call scope, this means each reference will contain
	// different values
	if fixtures.Uid1 == fixtures.Uid2 {
		t.FailNow()
		return
	}

	if !info.IsDir() {
		t.FailNow()
		return
	}
}

func TestSampleTests(t *testing.T) {
	gtest.RunSubTests(t, &SampleTests{})
}
