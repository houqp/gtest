// Package gtest provides the following functionalities to help reduce
// boilerplate in test code:
//
// - Test grouping
//
// - Setup, Teardown hooks for test groups
//
// - BeforeEach, AfterEach hooks for tests
//
// - Fixture injection
//
// Tests are grouped using struct methods. Each test in a test group needs to
// be defined as a struct method with `SubTest` prefix. To run a group of
// tests, have the test group struct implement `GTest` interface, then passe it
// to `RunSubTests` call.
//
// Example of test grouping:
//
//    import (
//      "strings"
//      "testing"
//      "github.com/houqp/gtest"
//    )
//
//    type SampleTests struct{}
//
//    // Setup and Teardown are invoked per test group run
//    func (s *SampleTests) Setup(t *testing.T)      {}
//    func (s *SampleTests) Teardown(t *testing.T)   {}
//    // BeforeEach and AfterEach are invoked per test run
//    func (s *SampleTests) BeforeEach(t *testing.T) {}
//    func (s *SampleTests) AfterEach(t *testing.T)  {}
//
//    func (s *SampleTests) SubTestCompare(t *testing.T) {
//      if 1 != 1 {
//        t.FailNow()
//      }
//    }
//
//    func (s *SampleTests) SubTestCheckPrefix(t *testing.T) {
//      if !strings.HasPrefix("abc", "ab") {
//        t.FailNow()
//      }
//    }
//
//    func TestSampleTests(t *testing.T) {
//      gtest.RunSubTests(t, &SampleTests{})
//    }
//
// Any struct with Construct and Destruct method defined can be registered as a
// fixture. After a fixture is registered, it can be referenced using fixture
// struct field tags.
//
// Example of fixture injection:
//
//    import (
//      "io/ioutil"
//      "os"
//      "testing"
//      "github.com/houqp/gtest"
//    )
//
//    type WorkDirFixture struct{}
//
//    // Construct can take other fixtures as input parameter as well
//    func (s WorkDirFixture) Construct(t *testing.T, fixtures struct{}) (string, string) {
//      dir, err := ioutil.TempDir("", "gtest-fixture")
//      assert.NoError(t, err)
//
//      // First return value will be passed to test as fixture value, second return value
//      // will be passed to Destruct as ctx for cleanup purpose if needed.
//      return dir, dir
//    }
//
//    // type for second input parameter of Destruct needs to match second return value of
//    // Construct method
//    func (s WorkDirFixture) Destruct(t *testing.T, dir string) {
//      os.RemoveAll(dir)
//    }
//
//    func init() {
//      gtest.MustRegisterFixture("WorkDir", &WorkDirFixture{}, gtest.ScopeSubTest)
//    }
//
//    type SampleTests struct{}
//
//    func (s *SampleTests) Setup(t *testing.T)      {}
//    func (s *SampleTests) Teardown(t *testing.T)   {}
//    func (s *SampleTests) BeforeEach(t *testing.T) {}
//    func (s *SampleTests) AfterEach(t *testing.T)  {}
//
//    func (s *SampleTests) SubTestMultipleFixtures(t *testing.T, fixtures struct {
//      DirPath string `fixture:"WorkDir"`
//    }) {
//      info, err := os.Stat(fixtures.DirPath)
//      if err != nil {
//        t.FailNow()
//        return
//      }
//    }
//
//    func TestSampleTests(t *testing.T) {
//      gtest.RunSubTests(t, &SampleTests{})
//    }
//
// Note that you can pass fixtures to fixture's Construct method as well, making it possible
// to build fixtures using other fixtures in a nested fashion.
package gtest
