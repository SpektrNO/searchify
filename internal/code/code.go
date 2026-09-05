package code

// Unit is a retrieval chunk boundary (function, class, method, or module preamble).
type Unit struct {
	Kind      string `json:"kind"` // function|async_function|class|method|module
	Name      string `json:"name"`
	QualName  string `json:"qual_name"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	ByteStart int    `json:"byte_start"`
	ByteEnd   int    `json:"byte_end"`
}

// Symbol is a definition site.
type Symbol struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	QualName string `json:"qual_name"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
	Col      int    `json:"col"`
}

// Ref is a best-effort reference (import, call, or name load).
type Ref struct {
	Kind     string `json:"kind"` // import|call|name
	Name     string `json:"name"`
	QualName string `json:"qual_name"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
}

// Result is the full analyze output for one file.
type Result struct {
	Units   []Unit   `json:"units"`
	Symbols []Symbol `json:"symbols"`
	Refs    []Ref    `json:"refs"`
}

// Analyzer extracts code units and symbols for a language.
type Analyzer interface {
	Lang() string
	Exts() []string
	Analyze(path string, src []byte) (Result, error)
}

var analyzers []Analyzer

func Register(a Analyzer) {
	if a == nil {
		return
	}
	analyzers = append(analyzers, a)
}

// ForPath returns an analyzer for the file extension, or nil.
func ForPath(path string) Analyzer {
	ext := extOf(path)
	if ext == "" {
		return nil
	}
	for _, a := range analyzers {
		for _, e := range a.Exts() {
			if e == ext {
				return a
			}
		}
	}
	return nil
}

func extOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		c := path[i]
		if c == '.' {
			return path[i:]
		}
		if c == '/' || c == '\\' {
			break
		}
	}
	return ""
}

func init() {
	Register(PythonAnalyzer{})
}
