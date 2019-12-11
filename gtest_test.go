package gtest_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/houqp/gtest"
	"github.com/stretchr/testify/assert"
)

// start of fixtures

// user id fixture that generates globally unique user ids
type UserIdFixture struct {
	Count int64

	// to simulate fixture instance allocation/deallocation
	AllocatedCount int
}

func (s *UserIdFixture) Construct(t *testing.T, fixtures struct{}) (string, interface{}) {
	s.AllocatedCount += 1
	s.Count += 1
	return fmt.Sprintf("user_%d", s.Count), nil
}

func (s *UserIdFixture) Destruct(t *testing.T, ctx interface{}) {
	s.AllocatedCount -= 1
}

// user fixture that generates MockUser structs with unique ids
type MockUser struct {
	Id string
}

type MockUserFixture struct {
	AllocatedCount int
}

func (s *MockUserFixture) Construct(t *testing.T, fixtures struct {
	UserId string `fixture:"UserId"`
}) (MockUser, interface{}) {
	s.AllocatedCount += 1
	return MockUser{
		Id: fixtures.UserId,
	}, nil
}
func (s *MockUserFixture) Destruct(t *testing.T, ctx interface{}) {
	s.AllocatedCount -= 1
}

// comment fixture that generates MockComment with unique creator
type MockComment struct {
	Creator MockUser
	Content string
	Created time.Time
}

type MockCommentFixture struct {
	AllocatedCount int
}

func (s *MockCommentFixture) Construct(t *testing.T, fixtures struct {
	User MockUser `fixture:"MockUser"`
}) (MockComment, interface{}) {
	s.AllocatedCount += 1
	return MockComment{
		Creator: fixtures.User,
		Content: "Hello GTest!",
		Created: time.Now(),
	}, nil
}

func (s *MockCommentFixture) Destruct(t *testing.T, ctx interface{}) {
	s.AllocatedCount -= 1
}

// temp dir fixture
type TmpDirFixture struct{}

func (s TmpDirFixture) Construct(t *testing.T, fixtures struct{}) (string, string) {
	dir, err := ioutil.TempDir("", "gtest")
	assert.NoError(t, err)
	return dir, dir
}

func (s TmpDirFixture) Destruct(t *testing.T, ctx interface{}) {
	dir := ctx.(string)
	os.RemoveAll(dir)
}

// test HTTP API server fixture
type MockApiServerFixture struct{}

type ApiServerFixtureCtx struct {
	Server   *httptest.Server
	DataFile *os.File
}

func (s MockApiServerFixture) Construct(t *testing.T, fixtures struct{}) (*httptest.Server, ApiServerFixtureCtx) {
	fp, err := ioutil.TempFile("", "gtest-api-server-healthcheck")
	assert.NoError(t, err)
	_, err = fp.Write([]byte("GTest server"))
	assert.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadFile(fp.Name())
		assert.NoError(t, err)
		fmt.Fprintln(w, "Hello, "+string(data))
	}))

	return ts, ApiServerFixtureCtx{
		Server:   ts,
		DataFile: fp,
	}
}

func (s MockApiServerFixture) Destruct(t *testing.T, ctx interface{}) {
	c := ctx.(ApiServerFixtureCtx)
	c.Server.Close()
	os.Remove(c.DataFile.Name())
}

// register all fixtures
func init() {
	gtest.MustRegisterFixture("UserId", &UserIdFixture{}, gtest.ScopeCall)
	gtest.MustRegisterFixture("MockUser", &MockUserFixture{}, gtest.ScopeCall)
	gtest.MustRegisterFixture("MockComment", &MockCommentFixture{}, gtest.ScopeCall)
	gtest.MustRegisterFixture("TmpDir", &TmpDirFixture{}, gtest.ScopeSubTest)
	gtest.MustRegisterFixture("MockApiServer", &MockApiServerFixture{}, gtest.ScopeSubTest)
}

// start of tests

type GTestTests struct {
	Initialized bool
}

func (s *GTestTests) Setup(t *testing.T) {
	s.Initialized = true
}

func (s *GTestTests) Teardown(t *testing.T) {
	s.Initialized = false
}

func (s *GTestTests) BeforeEach(t *testing.T) {
}

func (s *GTestTests) AfterEach(t *testing.T) {
	// automatically apply go routine leak check to all sub tests
	goleak.VerifyNone(t)
}

// simple test without fixture
func (GTestTests) SubTestRunTest(t *testing.T) {
	assert.Equal(t, 2, 1+1)
}

// call scoped fixture returns different values for each field
func (GTestTests) SubTestCallScopedStringFixture(t *testing.T, fixtures struct {
	UserId1 string `fixture:"UserId"`
	UserId2 string `fixture:"UserId"`
	UserId3 string `fixture:"UserId"`
}) {
	assert.True(t, strings.HasPrefix(fixtures.UserId1, "user_"))
	assert.True(t, strings.HasPrefix(fixtures.UserId2, "user_"))
	assert.True(t, strings.HasPrefix(fixtures.UserId3, "user_"))
	assert.NotEqual(t, fixtures.UserId1, fixtures.UserId2)
	assert.NotEqual(t, fixtures.UserId1, fixtures.UserId3)
	assert.NotEqual(t, fixtures.UserId2, fixtures.UserId3)
}

// fixture can be built using other fixtures as well
func (GTestTests) SubTestNestedFixture(t *testing.T, fixtures struct {
	Comment MockComment `fixture:"MockComment"`
}) {
	assert.True(t, strings.HasPrefix(fixtures.Comment.Creator.Id, "user_"))
	assert.True(t, strings.HasPrefix(fixtures.Comment.Content, "Hello GTest!"))
	assert.True(t, time.Now().Sub(fixtures.Comment.Created) < time.Second)
}

// For subtest scoped fixture, fixture construct will only be called once
// and result will be cached for subsequent calls within the subtest
func (GTestTests) SubTestSubTestScopedFixture(t *testing.T, fixtures struct {
	Dir1 string `fixture:"TmpDir"`
	Dir2 string `fixture:"TmpDir"`
	Dir3 string `fixture:"TmpDir"`
}) {
	assert.Equal(t, fixtures.Dir1, fixtures.Dir2)
	assert.Equal(t, fixtures.Dir1, fixtures.Dir3)

	info, err := os.Stat(fixtures.Dir1)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

// Fixture's Destruct method will be called after subtest finishes
func (GTestTests) SubTestFixtureDestruct(t *testing.T, fixtures struct {
	Srv *httptest.Server `fixture:"MockApiServer"`
}) {
	res, err := http.Get(fixtures.Srv.URL)
	assert.NoError(t, err)

	greeting, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err)
	res.Body.Close()

	assert.Equal(t, "Hello, GTest server\n", string(greeting))

	// goleak will report error is ApiServer fixture is not destructed properly
}

func (s *GTestTests) SubTestSetupAndTearDown(t *testing.T) {
	// s.Initialized will be checked again after RunSubTests returns in TestGTest
	assert.True(t, s.Initialized)
}

type InvalidFixtureMissingConstruct struct{}

type InvalidFixtureConstructInput struct{}

func (InvalidFixtureConstructInput) Construct() {}

type InvalidFixtureConstructInput1 struct{}

func (InvalidFixtureConstructInput1) Construct(t *testing.T, fixtures string) {}

type InvalidFixtureConstructInput2 struct{}

func (InvalidFixtureConstructInput2) Construct(t *testing.T, fixtures string) (string, interface{}) {
	return "", nil
}

type InvalidFixtureConstructInput3 struct{}

func (InvalidFixtureConstructInput3) Construct(t int, fixtures struct{}) (string, interface{}) {
	return "", nil
}

type InvalidFixtureMissingDestruct struct{}

func (InvalidFixtureMissingDestruct) Construct(t *testing.T, fixtures struct{}) (string, interface{}) {
	return "", nil
}

type InvalidFixtureDestructInput struct{}

func (InvalidFixtureDestructInput) Construct(t *testing.T, fixtures struct{}) (string, interface{}) {
	return "", nil
}
func (InvalidFixtureDestructInput) Destruct(t string) {}

type InvalidFixtureDestructOutput1 struct{}

func (InvalidFixtureDestructOutput1) Construct(t *testing.T, fixtures struct{}) (string, interface{}) {
	return "", nil
}
func (InvalidFixtureDestructOutput1) Destruct(t string, ctx interface{}) {}

type InvalidFixtureDestructOutput2 struct{}

func (InvalidFixtureDestructOutput2) Construct(t *testing.T, fixtures struct{}) (string, interface{}) {
	return "", nil
}
func (InvalidFixtureDestructOutput2) Destruct(t *testing.T, ctx struct{}) {}

func (s *GTestTests) SubTestInvalidFixtureRegistration(t *testing.T) {
	for _, f := range []interface{}{
		InvalidFixtureMissingConstruct{},
		InvalidFixtureConstructInput{},
		InvalidFixtureConstructInput1{},
		InvalidFixtureConstructInput2{},
		InvalidFixtureConstructInput3{},
		InvalidFixtureMissingDestruct{},
		InvalidFixtureDestructInput{},
		InvalidFixtureDestructOutput1{},
		InvalidFixtureDestructOutput2{},
	} {
		err := gtest.RegisterFixture("Foo", f, gtest.ScopeCall)
		assert.Error(t, err)
	}
}

func (s *GTestTests) SubTestFixtureDuplicatedReg(t *testing.T) {
	err := gtest.RegisterFixture("UserId", &UserIdFixture{}, gtest.ScopeSubTest)
	assert.Error(t, err)
}

func TestGTest(t *testing.T) {
	testGroup := &GTestTests{}
	assert.False(t, testGroup.Initialized)

	gtest.RunSubTests(t, testGroup)

	userIdFixture, _ := gtest.GetFixture("UserId")
	assert.Equal(t, 0, userIdFixture.Instance.(*UserIdFixture).AllocatedCount)
	userFixture, _ := gtest.GetFixture("MockUser")
	assert.Equal(t, 0, userFixture.Instance.(*MockUserFixture).AllocatedCount)
	commentFixture, _ := gtest.GetFixture("MockComment")
	assert.Equal(t, 0, commentFixture.Instance.(*MockCommentFixture).AllocatedCount)

	// test Teardown method
	assert.False(t, testGroup.Initialized)
}
