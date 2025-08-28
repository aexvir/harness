package harness

import (
	"context"
	"fmt"
	"reflect"
)

// TasksFrom returns all exported methods of a namespace as slice of [Task].
// The namespace type must be a struct where all exported methods
// follow the signature: func(ctx context.Context) error
//
// Example:
//
//	type Linter mg.Namespace
//
//	func (Linter) DoThis(ctx context.Context) error { ... }
//	func (Linter) DoThat(ctx context.Context) error { ... }
//
//	func Lint(ctx context.Context) error {
//	    return h.Execute(ctx, TasksFrom[Linter]()...)
//	}
func TasksFrom[t any]() []Task {
	var ns t

	tasks, err := tasksFromNamespace(ns)
	if err != nil {
		return nil
	}

	return tasks
}

// AsTasks wraps a list of methods as tasks that can be passed to [Execute].
// This allows running selected methods in a specific order, potentially from different namespaces.
//
// Example:
//
//	func Lint(ctx context.Context) error {
//		return h.Execute(
//			ctx,
//			harness.AsTasks(
//				Linter.GoModTidy,
//				Linter.Commitsar,
//				Checker.ValidateCode,  // from different namespace
//				Cleanup,               // not namespaced task
//			)...
//		)
//	}
func AsTasks(methods ...any) []Task {
	tasks, err := tasksFromMethods(methods)
	if err != nil {
		return nil
	}

	return tasks
}

// tasksFromNamespace uses reflection to find all methods on the given namespace
// that match the [Task] signature and returns them as a slice of Tasks.
func tasksFromNamespace(namespace any) ([]Task, error) {
	val, typ := reflect.ValueOf(namespace), reflect.TypeOf(namespace)

	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("namespace must be a struct, got %T", namespace)
	}

	var tasks []Task

	// iterate through all methods of the type
	for i := range typ.NumMethod() {
		method := typ.Method(i)

		// skip unexported
		if !method.IsExported() {
			continue
		}

		// skip methods whose signature doesn't match a namespaced task function
		// signature: func(receiver any, ctx context.Context) error
		if !isNamespacedTaskFunc(method.Type) {
			continue
		}

		tasks = append(
			tasks,
			func(ctx context.Context) error {
				return mustReturnError(
					// receiver is already "registered" as we're looping the methods of an instanced namespace
					val.Method(i).Call([]reflect.Value{reflect.ValueOf(ctx)}),
				)
			},
		)
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks found in namespace %T", namespace)
	}

	return tasks, nil
}

// tasksFromMethods converts method expressions to Tasks
func tasksFromMethods(methods []any) ([]Task, error) {
	var tasks []Task

	for _, method := range methods {
		val := reflect.ValueOf(method)
		typ := val.Type()

		var task Task

		switch typ.NumIn() {
		case 1:
			if !isTaskFunc(typ) {
				return nil, fmt.Errorf("invalid signature for %T: only tasks and namespaced tasks are supported", method)
			}

			task = func(ctx context.Context) error {
				return mustReturnError(
					val.Call([]reflect.Value{reflect.ValueOf(ctx)}),
				)
			}

		case 2:
			if !isNamespacedTaskFunc(typ) {
				return nil, fmt.Errorf("invalid signature for %T: only tasks and namespaced tasks are supported", method)
			}

			receiver := reflect.New(typ.In(0)).Elem()
			task = func(ctx context.Context) error {
				return mustReturnError(
					val.Call([]reflect.Value{receiver, reflect.ValueOf(ctx)}),
				)
			}

		default:
			return nil, fmt.Errorf("invalid signature: expected 1 or 2 parameters, got %d", typ.NumIn())
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// isTaskFunc returns true if the received [reflect.Type] is a function
// with the correct signature of a [Task].
func isTaskFunc(t reflect.Type) bool {
	isFunctionType := t.Kind() == reflect.Func
	hasOneArgument := t.NumIn() == 1
	hasOneReturnType := t.NumOut() == 1

	// verify that received type is a function with the correct signature
	// simple harness tasks should have 1 argument (context) and 1 return (error)
	if !isFunctionType || !hasOneArgument || !hasOneReturnType {
		return false
	}

	// extract argument and return type
	// verify that argument is context and return is an error
	arg, ret := t.In(0), t.Out(0)

	isArgumentContext := arg == reflect.TypeFor[context.Context]()
	isReturnError := ret == reflect.TypeFor[error]()

	if isArgumentContext && isReturnError {
		return true
	}

	return false
}

// isHarnessTaskFunc returns true if the received [reflect.Type] is a function
// with the correct signature of a namespaced [Task].
func isNamespacedTaskFunc(t reflect.Type) bool {
	isFunctionType := t.Kind() == reflect.Func
	hasTwoArguments := t.NumIn() == 2
	hasOneReturnType := t.NumOut() == 1

	// verify that received type is a function with the correct signature
	// namespaced harness tasks should have 2 params (receiver + context) and 1 return (error)
	if !isFunctionType || !hasTwoArguments || !hasOneReturnType {
		return false
	}

	// extract receiver, argument and return type
	// verify that receiver is a struct, argument is context and return is an error
	receiver, arg, ret := t.In(0), t.In(1), t.Out(0)

	isReceiverStruct := receiver.Kind() == reflect.Struct
	isSecondArgumentContext := arg == reflect.TypeFor[context.Context]()
	isReturnError := ret == reflect.TypeFor[error]()

	if isReceiverStruct && isSecondArgumentContext && isReturnError {
		return true
	}

	return false
}

// mustReturnError extracts the returned error from a function call.
// panics if the function returns more than one type or if the returned type is not an error.
// handles nil errors just fine.
// should be safe to call after [isTaskFunc] or [isNamespacedTaskFunc] have verified the function signature.
func mustReturnError(ret []reflect.Value) error {
	if len(ret) != 1 {
		panic(fmt.Sprintf("function expected to have single return error type, got %d instead", len(ret)))
	}

	if ret[0].IsNil() {
		return nil
	}

	err, ok := reflect.TypeAssert[error](ret[0])
	if !ok {
		panic(fmt.Sprintf("return type should be error, got %T instead", err))
	}
	return err
}
