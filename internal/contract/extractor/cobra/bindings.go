package cobra

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// FlagBinding represents a single flag registration call
type FlagBinding struct {
	FlagName string       // The flag name (e.g., "keyword")
	FlagType string       // The flag type (e.g., "String", "Int", "Bool")
	BindCall *ssa.Call    // The SSA call instruction
	VarAddr  ssa.Value    // The variable address passed to XxxVar (Alloc/Global/FieldAddr)
	OwnerCmd string       // Command identifier (for debugging)
	Owner    BindingOwner // Part 2: identifies binding ownership
}

// VarKind represents the kind of variable address
type VarKind int

const (
	VarUnknown   VarKind = iota
	VarAlloc             // Local variable: var x
	VarGlobal            // Package-level variable: var x
	VarFieldAddr         // Struct field: &owner.field
)

// BindingOwner identifies the owner of a flag binding
type BindingOwner struct {
	Kind    string        // "global", "factory", "unknown"
	Global  *ssa.Global   // Set when Kind == "global"
	Factory *ssa.Function // Set when Kind == "factory"
}

// flagBindingMethods maps pflag method names to their type names (72 items total)
var flagBindingMethods = map[string]string{
	// String variants
	"StringVar":          "String",
	"StringVarP":         "String",
	"StringSliceVar":     "StringSlice",
	"StringSliceVarP":    "StringSlice",
	"StringArrayVar":     "StringArray",
	"StringArrayVarP":    "StringArray",
	"StringToStringVar":  "StringToString",
	"StringToStringVarP": "StringToString",
	"StringToIntVar":     "StringToInt",
	"StringToIntVarP":    "StringToInt",
	"StringToInt64Var":   "StringToInt64",
	"StringToInt64VarP":  "StringToInt64",

	// Bool variants
	"BoolVar":       "Bool",
	"BoolVarP":      "Bool",
	"BoolSliceVar":  "BoolSlice",
	"BoolSliceVarP": "BoolSlice",

	// Int variants
	"IntVar":         "Int",
	"IntVarP":        "Int",
	"Int8Var":        "Int8",
	"Int8VarP":       "Int8",
	"Int16Var":       "Int16",
	"Int16VarP":      "Int16",
	"Int32Var":       "Int32",
	"Int32VarP":      "Int32",
	"Int64Var":       "Int64",
	"Int64VarP":      "Int64",
	"IntSliceVar":    "IntSlice",
	"IntSliceVarP":   "IntSlice",
	"Int32SliceVar":  "Int32Slice",
	"Int32SliceVarP": "Int32Slice",
	"Int64SliceVar":  "Int64Slice",
	"Int64SliceVarP": "Int64Slice",

	// Uint variants
	"UintVar":       "Uint",
	"UintVarP":      "Uint",
	"Uint8Var":      "Uint8",
	"Uint8VarP":     "Uint8",
	"Uint16Var":     "Uint16",
	"Uint16VarP":    "Uint16",
	"Uint32Var":     "Uint32",
	"Uint32VarP":    "Uint32",
	"Uint64Var":     "Uint64",
	"Uint64VarP":    "Uint64",
	"UintSliceVar":  "UintSlice",
	"UintSliceVarP": "UintSlice",

	// Float variants
	"Float32Var":       "Float32",
	"Float32VarP":      "Float32",
	"Float32SliceVar":  "Float32Slice",
	"Float32SliceVarP": "Float32Slice",
	"Float64Var":       "Float64",
	"Float64VarP":      "Float64",
	"Float64SliceVar":  "Float64Slice",
	"Float64SliceVarP": "Float64Slice",

	// Duration variants
	"DurationVar":       "Duration",
	"DurationVarP":      "Duration",
	"DurationSliceVar":  "DurationSlice",
	"DurationSliceVarP": "DurationSlice",

	// Other types
	"CountVar":        "Count",
	"CountVarP":       "Count",
	"BytesHexVar":     "BytesHex",
	"BytesHexVarP":    "BytesHex",
	"BytesBase64Var":  "BytesBase64",
	"BytesBase64VarP": "BytesBase64",

	// IP variants
	"IPVar":       "IP",
	"IPVarP":      "IP",
	"IPSliceVar":  "IPSlice",
	"IPSliceVarP": "IPSlice",
	"IPNetVar":    "IPNet",
	"IPNetVarP":   "IPNet",
	"IPMaskVar":   "IPMask",
	"IPMaskVarP":  "IPMask",

	// Generic Var
	"Var":   "Value",
	"VarP":  "Value",
	"VarPF": "Value",
}

// findFlagBindings finds all flag binding calls in a function
func findFlagBindings(fn *ssa.Function) []FlagBinding {
	var bindings []FlagBinding

	if fn == nil || fn.Blocks == nil {
		return bindings
	}

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(*ssa.Call)
			if !ok {
				continue
			}

			flagType, ok := isFlagBindingMethod(call)
			if !ok {
				continue
			}

			// Extract flag name (second string argument typically)
			flagName := findFirstStringConstArg(call)

			// Extract variable address (first pointer argument after FlagSet)
			varAddr := findFirstPointerArg(call)

			// Resolve owner (Part 2)
			owner := resolveBindingOwner(call)

			if varAddr != nil {
				bindings = append(bindings, FlagBinding{
					FlagName: flagName,
					FlagType: flagType,
					BindCall: call,
					VarAddr:  varAddr,
					Owner:    owner,
				})
			}
		}
	}

	return bindings
}

// isFlagBindingMethod checks if a call is a pflag binding method
func isFlagBindingMethod(call *ssa.Call) (flagType string, ok bool) {
	if call == nil || call.Call.Value == nil {
		return "", false
	}

	callee, ok := call.Call.Value.(*ssa.Function)
	if !ok || callee == nil {
		return "", false
	}

	// Check if it's a pflag method
	if callee.Pkg == nil || callee.Pkg.Pkg == nil {
		return "", false
	}

	if callee.Pkg.Pkg.Path() != "github.com/spf13/pflag" {
		return "", false
	}

	// Check if receiver is *FlagSet
	if callee.Signature == nil || callee.Signature.Recv() == nil {
		return "", false
	}

	if !isFlagSetPointer(callee.Signature.Recv().Type()) {
		return "", false
	}

	// Check method name against whitelist
	methodName := callee.Name()
	flagType, ok = flagBindingMethods[methodName]
	return flagType, ok
}

// isFlagSetPointer checks if a type is *pflag.FlagSet
func isFlagSetPointer(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}

	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil {
		return false
	}

	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}

	return pkg.Path() == "github.com/spf13/pflag" && obj.Name() == "FlagSet"
}

// findFirstPointerArg finds the first pointer argument that is not a FlagSet
// This is the variable address for XxxVar methods
func findFirstPointerArg(call *ssa.Call) ssa.Value {
	if call == nil {
		return nil
	}

	for _, arg := range call.Call.Args {
		if arg == nil {
			continue
		}

		// Check if argument type is pointer
		if _, ok := arg.Type().(*types.Pointer); !ok {
			continue
		}

		// Skip FlagSet pointers (receiver is already stripped)
		if isFlagSetPointer(arg.Type()) {
			continue
		}

		// This is the variable address
		return arg
	}

	return nil
}

// findFirstStringConstArg finds the first string constant argument
// This is typically the flag name
func findFirstStringConstArg(call *ssa.Call) string {
	if call == nil {
		return ""
	}

	for _, arg := range call.Call.Args {
		if s := extractStringConst(arg); s != "" {
			return s
		}
	}

	return ""
}

// extractStringConst extracts a string constant from an SSA value
// Handles MakeInterface, ChangeType, Convert wrappers
func extractStringConst(v ssa.Value) string {
	if v == nil {
		return ""
	}

	// Unwrap interface/type conversions
	for {
		switch val := v.(type) {
		case *ssa.Const:
			if val.Value != nil && val.Type() != nil {
				if basic, ok := val.Type().Underlying().(*types.Basic); ok {
					if basic.Kind() == types.String {
						// Remove quotes from string constant
						s := val.Value.ExactString()
						if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
							return s[1 : len(s)-1]
						}
						return s
					}
				}
			}
			return ""
		case *ssa.MakeInterface:
			v = val.X
		case *ssa.ChangeType:
			v = val.X
		case *ssa.Convert:
			v = val.X
		default:
			return ""
		}
	}
}

// classifyVarAddr classifies the kind of variable address
func classifyVarAddr(v ssa.Value) VarKind {
	if v == nil {
		return VarUnknown
	}

	switch v.(type) {
	case *ssa.Alloc:
		return VarAlloc
	case *ssa.Global:
		return VarGlobal
	case *ssa.FieldAddr:
		return VarFieldAddr
	default:
		return VarUnknown
	}
}

// varAddrName returns a human-readable name for a variable address
func varAddrName(v ssa.Value) string {
	if v == nil {
		return "<nil>"
	}

	switch val := v.(type) {
	case *ssa.Alloc:
		if val.Comment != "" {
			return val.Comment
		}
		return val.Name()
	case *ssa.Global:
		return val.Name()
	case *ssa.FieldAddr:
		// Get field name from struct type
		t := val.X.Type()
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		}
		if structType, ok := t.Underlying().(*types.Struct); ok {
			if val.Field >= 0 && val.Field < structType.NumFields() {
				ownerName := varAddrName(val.X)
				fieldName := structType.Field(val.Field).Name()
				return ownerName + "." + fieldName
			}
		}
		return "<field>"
	default:
		return val.Name()
	}
}

// resolveBindingOwner traces a flag binding call back to its command owner
func resolveBindingOwner(call *ssa.Call) BindingOwner {
	if call == nil || len(call.Call.Args) == 0 {
		return BindingOwner{Kind: "unknown"}
	}

	// The first argument is the FlagSet receiver
	flagSet := call.Call.Args[0]

	// FlagSet typically comes from cmd.Flags() or cmd.PersistentFlags()
	flagsCall, ok := flagSet.(*ssa.Call)
	if !ok {
		return BindingOwner{Kind: "unknown"}
	}

	// Verify it's a Flags() or PersistentFlags() method
	if !isCommandFlagsMethod(flagsCall) {
		return BindingOwner{Kind: "unknown"}
	}

	// The receiver of Flags() is the command
	if len(flagsCall.Call.Args) == 0 {
		return BindingOwner{Kind: "unknown"}
	}

	cmdReceiver := flagsCall.Call.Args[0]

	// Determine the kind of command receiver
	switch recv := cmdReceiver.(type) {
	case *ssa.UnOp:
		// UnOp{MUL, Global} -> Pattern W: global command
		if recv.X != nil {
			if g, ok := recv.X.(*ssa.Global); ok {
				return BindingOwner{Kind: "global", Global: g}
			}
		}
	case *ssa.Alloc:
		// Alloc -> Pattern A: factory function
		if recv.Parent() != nil {
			return BindingOwner{Kind: "factory", Factory: recv.Parent()}
		}
	case *ssa.Parameter:
		// Parameter -> helper function pattern (not fully supported in Part 2)
		return BindingOwner{Kind: "unknown"}
	}

	return BindingOwner{Kind: "unknown"}
}

// isCommandFlagsMethod checks if a call is to cobra.Command.Flags() or PersistentFlags()
func isCommandFlagsMethod(call *ssa.Call) bool {
	if call == nil || call.Call.Value == nil {
		return false
	}

	callee, ok := call.Call.Value.(*ssa.Function)
	if !ok || callee == nil {
		return false
	}

	// Check if it's a method on *cobra.Command
	if callee.Signature == nil || callee.Signature.Recv() == nil {
		return false
	}

	if !isCobraCommandPointer(callee.Signature.Recv().Type()) {
		return false
	}

	// Check method name
	name := callee.Name()
	return name == "Flags" || name == "PersistentFlags"
}

// findFlagBindingsForGlobal finds all flag bindings for a global command across the program
func findFlagBindingsForGlobal(prog *ssa.Program, globalCmd *ssa.Global) []FlagBinding {
	var bindings []FlagBinding

	if prog == nil || globalCmd == nil {
		return bindings
	}

	// Scan all functions in all packages
	for _, pkg := range prog.AllPackages() {
		if pkg == nil {
			continue
		}

		for _, mem := range pkg.Members {
			fn, ok := mem.(*ssa.Function)
			if !ok || fn == nil || fn.Blocks == nil {
				continue
			}

			// Scan function for flag bindings
			for _, block := range fn.Blocks {
				for _, instr := range block.Instrs {
					call, ok := instr.(*ssa.Call)
					if !ok {
						continue
					}

					flagType, ok := isFlagBindingMethod(call)
					if !ok {
						continue
					}

					// Resolve owner
					owner := resolveBindingOwner(call)
					if owner.Kind != "global" || owner.Global != globalCmd {
						continue
					}

					// This binding belongs to our global command
					flagName := findFirstStringConstArg(call)
					varAddr := findFirstPointerArg(call)

					if varAddr != nil {
						bindings = append(bindings, FlagBinding{
							FlagName: flagName,
							FlagType: flagType,
							BindCall: call,
							VarAddr:  varAddr,
							Owner:    owner,
						})
					}
				}
			}
		}
	}

	return bindings
}
