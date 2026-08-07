package main

import "strings"

// rearrangeFlags moves flags before operands so Go's flag package sees them.
// FlagSet stops at the first non-flag, so `index C:\vault --skip-embed` used to
// ignore --skip-embed and still load the embedder (multi-GB RSS).
//
// valueFlags lists flag names (without leading dashes) that consume the next arg
// when not written as --name=value.
func rearrangeFlags(args []string, valueFlags map[string]struct{}) []string {
	var flags, operands []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			operands = append(operands, a)
			continue
		}

		flags = append(flags, a)
		if strings.Contains(a, "=") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		if _, ok := valueFlags[name]; !ok {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			flags = append(flags, args[i])
		}
	}
	out := make([]string, 0, len(flags)+len(operands))
	out = append(out, flags...)
	out = append(out, operands...)
	return out
}

func rejectDanglingFlags(args []string) error {
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			return errDanglingFlag(a)
		}
	}
	return nil
}

type errDanglingFlag string

func (e errDanglingFlag) Error() string {
	return "unexpected flag among paths: " + string(e) + " (put options before paths, e.g. index --skip-embed <path>)"
}
