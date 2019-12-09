package gtest

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/fatih/structtag"
)

type FixtureScope string

const (
	// ScopeSubTest fixture's value will be cached, so multiple reference in
	// the same subtest will result in the same cached value.
	//
	// Good usecase for this scope is injecting the same database transaction
	// used by all fixtures in a subtest.
	ScopeSubTest FixtureScope = "subtest"
	// ScopeCall fixture will return different value for each reference in fixtures struct.
	ScopeCall FixtureScope = "call"

	testMethodPrefix = "SubTest"
)

// Subtests are grouped in struct that confirms to GTest interface
type GTest interface {
	// Setup is called before any subtest runs in a test group.
	Setup(t *testing.T)
	// Teardown is called after all subtests are completed in a test group.
	Teardown(t *testing.T)

	// BeforeEach is called before each subtest runs.
	BeforeEach(t *testing.T)
	// AfterEach is called after each subtest is completed.
	// A good use case is doing go routine leak check in this method.
	AfterEach(t *testing.T)
}

type FixtureEntry struct {
	Scope    FixtureScope
	Instance interface{}
}

var registeredFixtures map[string]FixtureEntry = map[string]FixtureEntry{}

func validateFixtureConstructMethod(fType reflect.Type) error {
	constructMethod, ok := fType.MethodByName("Construct")
	if !ok {
		return fmt.Errorf("%s missing required Construct method.", fType.String())
	}

	if constructMethod.Type.NumIn() != 3 {
		return fmt.Errorf("%s's Construct method needs to take exactly 2 input parameter as fixtures struct, got: %d.", fType.String(), constructMethod.Type.NumIn()-1)
	}
	if constructMethod.Type.NumOut() != 2 {
		return fmt.Errorf("%s's Construct method needs to return exactly 2 output parameters as as value and destruct context, got: %d.", fType.String(), constructMethod.Type.NumOut())
	}

	arg1 := constructMethod.Type.In(1)
	if arg1.String() != "*testing.T" {
		return fmt.Errorf("%s's Construct method needs to take *testing.T as first argument, got: %s", fType.String(), arg1.String())
	}

	arg2 := constructMethod.Type.In(2)
	if arg2.Kind() != reflect.Struct {
		return fmt.Errorf("%s's Construct method needs to take a struct as second argument", fType.String())
	}

	return nil
}

func validateFixtureDestructMethod(fType reflect.Type) error {
	destructMethod, ok := fType.MethodByName("Destruct")
	if !ok {
		return fmt.Errorf("%s missing required Destruct method.", fType.String())
	}
	if destructMethod.Type.NumIn() != 3 {
		return fmt.Errorf("%s's Destruct method needs to take exactly 2 input parameter as destruct context, got %d.", fType.String(), destructMethod.Type.NumIn()-1)
	}

	arg1 := destructMethod.Type.In(1)
	if arg1.String() != "*testing.T" {
		return fmt.Errorf("%s's Destruct method needs to take *testing.T as first argument, got: %s", fType.String(), arg1.String())
	}

	arg2 := destructMethod.Type.In(2)
	if arg2.String() != "interface {}" {
		return fmt.Errorf("%s's Destruct method needs to take interface {} as second argument, got: %s", fType.String(), arg2.String())
	}

	return nil
}

// Register a fixture under a given name. A fixture needs to be registered
// before it can be used in tests or other fixtures.
func RegisterFixture(name string, f interface{}, scope FixtureScope) error {
	entry, ok := registeredFixtures[name]
	if ok {
		return fmt.Errorf(
			"Fixture '%s' has already been registered under scope: %s",
			name, entry.Scope)
	}

	fType := reflect.TypeOf(f)

	err := validateFixtureConstructMethod(fType)
	if err != nil {
		return err
	}

	err = validateFixtureDestructMethod(fType)
	if err != nil {
		return err
	}

	registeredFixtures[name] = FixtureEntry{
		Scope:    scope,
		Instance: f,
	}
	return nil
}

// Register a fixture, panic if registration failed.
func MustRegisterFixture(name string, f interface{}, scope FixtureScope) {
	err := RegisterFixture(name, f, scope)
	if err != nil {
		panic(fmt.Sprintf("Failed to register fixture: %v", err))
	}
}

func GetFixture(name string) (FixtureEntry, bool) {
	val, ok := registeredFixtures[name]
	return val, ok
}

type fixtureResolver struct {
	// Resolved is keyed off registered fixture instance This means same.
	//
	// fixture instance registered under different names will be considered as
	// one.
	Resolved map[interface{}]reflect.Value
}

func newFixtureResolver() *fixtureResolver {
	f := fixtureResolver{
		Resolved: make(map[interface{}]reflect.Value),
	}
	return &f
}

func (self *fixtureResolver) resolve(t *testing.T, fixturesType reflect.Type, caller string, cleanUpCbs *[]func()) reflect.Value {
	kind := fixturesType.Kind()
	if kind != reflect.Struct {
		t.Fatalf("Invalid type for fixtures parameter, needs to be struct, got: %d", kind)
	}
	fixturesVal := reflect.Indirect(reflect.New(fixturesType))

	// iterate each field from fixtures struct
	for j := 0; j < fixturesType.NumField(); j++ {
		field := fixturesType.Field(j)
		fieldTagStr := string(field.Tag)
		tags, err := structtag.Parse(fieldTagStr)
		if err != nil {
			t.Fatalf("Invalid tag encountered for fixture: %s", fieldTagStr)
			continue
		}
		fixgureTag, err := tags.Get("fixture")
		if err != nil {
			t.Fatalf(
				"Struct field (%s %s) missing fixture tag for caller %s",
				field.Name, field.Type, caller)
			continue
		}
		fentry, ok := registeredFixtures[fixgureTag.Name]
		if !ok {
			t.Fatalf(
				"Unregistered fixture found for caller %s: %s", caller, *fixgureTag)
		}

		f := fentry.Instance

		var valVal reflect.Value
		ok = false
		switch fentry.Scope {
		case ScopeSubTest:
			valVal, ok = self.Resolved[f]
		}

		if !ok {
			// Type for fixture struct
			fType := reflect.TypeOf(f)
			// Value for fixture struct
			fVal := reflect.ValueOf(f)
			// input and output types are checked at runtime by RegisterFixture method
			constructMethod, _ := fType.MethodByName("Construct")
			constructType := constructMethod.Type
			callParams := []reflect.Value{
				reflect.ValueOf(t),
				self.resolve(
					t, constructType.In(2), fmt.Sprintf("%s.Construct", fType.String()), cleanUpCbs),
			}
			constructVal := fVal.MethodByName("Construct")
			returns := constructVal.Call(callParams)
			valVal = returns[0]
			ctxVal := returns[1]

			*cleanUpCbs = append(*cleanUpCbs, func() {
				destructVal := fVal.MethodByName("Destruct")
				destructVal.Call([]reflect.Value{
					reflect.ValueOf(t),
					ctxVal,
				})
			})

			if fentry.Scope == ScopeSubTest {
				self.Resolved[f] = valVal
			}
		}

		fixturesValField := fixturesVal.FieldByName(field.Name)
		if fixturesValField.CanSet() {
			fixturesValField.Set(valVal)
		} else {
			t.Fatalf("%s's fixture %s needs to be an exported struct field", caller, field.Name)
		}
	}

	return fixturesVal
}

// Run a group of sub tests.
func RunSubTests(t *testing.T, gt GTest) {
	// inspired by https://github.com/grpc/grpc-go/pull/2523/files
	xt := reflect.TypeOf(gt)
	xv := reflect.ValueOf(gt)

	gt.Setup(t)

	for i := 0; i < xt.NumMethod(); i++ {
		// use resolver to cache Fixture construct per test/method
		resolver := newFixtureResolver()

		method := xt.Method(i)
		methodName := method.Name
		if !strings.HasPrefix(methodName, testMethodPrefix) {
			continue
		}

		// method.Type.NumIn() includes struct itself into the count, but value.Call()
		// doesn't count struct as input parameter.
		methodParamCount := method.Type.NumIn() - 1

		callParams := make([]reflect.Value, methodParamCount)
		cleanUpCbs := []func(){}
		if methodParamCount < 1 {
			t.Fatalf("Method %v must have *testing.T as first parameter, got nothing.", methodName)
		}

		// first parameter should be testing.T
		argType := method.Type.In(1).String()
		if argType != "*testing.T" {
			t.Fatalf(
				"Method %v must have *testing.T as first parameter, got: %s",
				methodName, argType)
		}

		// second optional parameter should be fixtures struct
		if methodParamCount == 2 {
			fixturesType := method.Type.In(2)
			callParams[1] = resolver.resolve(t, fixturesType, methodName, &cleanUpCbs)
		} else if methodParamCount > 2 {
			t.Fatalf(
				"Method %s cannot take more than 2 parameters, got %d.",
				methodName, methodParamCount)
		}

		tfunc := xv.MethodByName(methodName)

		t.Run(strings.TrimPrefix(methodName, testMethodPrefix), func(t *testing.T) {
			gt.BeforeEach(t)

			callParams[0] = reflect.ValueOf(t)
			tfunc.Call(callParams)

			for _, cb := range cleanUpCbs {
				cb()
			}

			gt.AfterEach(t)
		})
	}

	gt.Teardown(t)
}
