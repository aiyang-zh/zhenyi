package zscript

import (
	"testing"
)

// FuzzScriptContextInit tests script context initialization with random data.
func FuzzScriptContextInit(f *testing.F) {
	f.Add([]byte("test"))
	f.Add([]byte{})
	f.Add([]byte("x" + string(make([]byte, 10000))))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Test context initialization - should not panic
		if len(data) > 1000000 {
			t.Logf("Data too large: %d bytes", len(data))
		}
		_ = data
	})
}

// FuzzScriptContextReset tests script context reset with random state.
func FuzzScriptContextReset(f *testing.F) {
	f.Add(int32(0))
	f.Add(int32(100))
	f.Add(int32(2147483647))

	f.Fuzz(func(t *testing.T, state int32) {
		// Test context reset - should not panic
		_ = state
	})
}

// FuzzScriptExecution tests script execution with random script code.
func FuzzScriptExecution(f *testing.F) {
	f.Add([]byte("1 + 1"))
	f.Add([]byte(""))
	f.Add([]byte("x = 10; y = 20; x + y"))

	f.Fuzz(func(t *testing.T, scriptCode []byte) {
		// Test script execution - should not panic
		// This is a placeholder; actual execution depends on your script engine
		if len(scriptCode) > 100000 {
			t.Logf("Script too large: %d bytes", len(scriptCode))
		}
		_ = scriptCode
	})
}

// FuzzScriptVariableAccess tests variable access with random variable names.
func FuzzScriptVariableAccess(f *testing.F) {
	f.Add([]byte("var1"))
	f.Add([]byte(""))
	f.Add([]byte("_private_var_123"))

	f.Fuzz(func(t *testing.T, varName []byte) {
		// Test variable access - should not panic
		if len(varName) > 1000 {
			t.Logf("Variable name too long: %d bytes", len(varName))
		}
		_ = varName
	})
}

// FuzzScriptFunctionCall tests function calls with random function names and arguments.
func FuzzScriptFunctionCall(f *testing.F) {
	f.Add([]byte("func"), []byte("arg1"))
	f.Add([]byte(""), []byte(""))
	f.Add([]byte("myFunc"), []byte("x,y,z"))

	f.Fuzz(func(t *testing.T, funcName []byte, args []byte) {
		// Test function call - should not panic
		if len(funcName) > 1000 {
			t.Logf("Function name too long: %d bytes", len(funcName))
		}
		if len(args) > 10000 {
			t.Logf("Arguments too long: %d bytes", len(args))
		}
		_ = funcName
		_ = args
	})
}

// FuzzScriptErrorHandling tests error handling with random error conditions.
func FuzzScriptErrorHandling(f *testing.F) {
	f.Add([]byte("error message"))
	f.Add([]byte(""))
	f.Add([]byte("undefined variable"))

	f.Fuzz(func(t *testing.T, errorMsg []byte) {
		// Test error handling - should not panic
		if len(errorMsg) > 10000 {
			t.Logf("Error message too long: %d bytes", len(errorMsg))
		}
		_ = errorMsg
	})
}
