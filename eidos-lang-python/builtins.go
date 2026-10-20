// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

// builtinClasses maps each public class of the module builtins of CPython
// 3.15 to whether it is an exception class, a class whose base chain
// reaches BaseException. EnvironmentError and IOError are names of OSError,
// and so is WindowsError on Windows.
var builtinClasses = map[string]bool{
	"bool": false, "bytearray": false, "bytes": false, "classmethod": false, "complex": false,
	"dict": false, "enumerate": false, "filter": false, "float": false, "frozendict": false,
	"frozenset": false, "int": false, "list": false, "map": false, "memoryview": false, "object": false,
	"property": false, "range": false, "reversed": false, "sentinel": false, "set": false, "slice": false,
	"staticmethod": false, "str": false, "super": false, "tuple": false, "type": false, "zip": false,
	"ArithmeticError": true, "AssertionError": true, "AttributeError": true, "BaseException": true,
	"BaseExceptionGroup": true, "BlockingIOError": true, "BrokenPipeError": true, "BufferError": true,
	"BytesWarning": true, "ChildProcessError": true, "ConnectionAbortedError": true,
	"ConnectionError": true, "ConnectionRefusedError": true, "ConnectionResetError": true,
	"DeprecationWarning": true, "EOFError": true, "EncodingWarning": true, "EnvironmentError": true,
	"Exception": true, "ExceptionGroup": true, "FileExistsError": true, "FileNotFoundError": true,
	"FloatingPointError": true, "FutureWarning": true, "GeneratorExit": true, "IOError": true,
	"ImportCycleError": true, "ImportError": true, "ImportWarning": true, "IndentationError": true,
	"IndexError": true, "InterruptedError": true, "IsADirectoryError": true, "KeyError": true,
	"KeyboardInterrupt": true, "LookupError": true, "MemoryError": true, "ModuleNotFoundError": true,
	"NameError": true, "NotADirectoryError": true, "NotImplementedError": true, "OSError": true,
	"OverflowError": true, "PendingDeprecationWarning": true, "PermissionError": true,
	"ProcessLookupError": true, "PythonFinalizationError": true, "RecursionError": true,
	"ReferenceError": true, "ResourceWarning": true, "RuntimeError": true, "RuntimeWarning": true,
	"StopAsyncIteration": true, "StopIteration": true, "SyntaxError": true, "SyntaxWarning": true,
	"SystemError": true, "SystemExit": true, "TabError": true, "TimeoutError": true, "TypeError": true,
	"UnboundLocalError": true, "UnicodeDecodeError": true, "UnicodeEncodeError": true,
	"UnicodeError": true, "UnicodeTranslateError": true, "UnicodeWarning": true, "UserWarning": true,
	"ValueError": true, "Warning": true, "WindowsError": true, "ZeroDivisionError": true,
}

// IsBuiltinClass reports whether name is a public class of the module
// builtins of CPython 3.15, such as int, property or ValueError. A function
// of builtins, such as len, is no class, and neither is a constant, such as
// NotImplemented. IsBuiltinClass allocates nothing.
func IsBuiltinClass(name string) bool {
	_, class := builtinClasses[name]
	return class
}

// IsBuiltinException reports whether name is a public exception class of the
// module builtins of CPython 3.15, a class whose base chain reaches
// BaseException, such as Exception or ValueError. IsBuiltinException
// allocates nothing.
func IsBuiltinException(name string) bool { return builtinClasses[name] }
