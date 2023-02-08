package printer

import "io"

// TruncateOutput defines should output be truncated
var TruncateOutput bool

// Printer defines capabilities to display content in kpt CLI.
// The main intention, at the moment, is to abstract away printing
// output in the CLI so that we can evolve the kpt CLI UX.
type Printer interface {
	//PrintPackage(pkg *pkg.Pkg, leadingNewline bool)
	Printf(format string, args ...interface{})
	//OptPrintf(opt *Options, format string, args ...interface{})
	OutStream() io.Writer
	ErrStream() io.Writer
}
