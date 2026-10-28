package mcp

import (
	"fmt"
	"strings"

	"github.com/spektr/searchify/internal/local"
)

// resolveSearchPathAllow turns optional path_prefixes / paths into a PathAllow.
// Both nil → unscoped (nil allow). Either non-nil (including empty) → constrained.
func (s *Server) resolveSearchPathAllow(prefixes, paths []string) (*local.PathAllow, error) {
	if prefixes == nil && paths == nil {
		return nil, nil
	}

	out := &local.PathAllow{}
	if err := appendResolvedPaths(s, prefixes, true, out); err != nil {
		return nil, err
	}
	if err := appendResolvedPaths(s, paths, false, out); err != nil {
		return nil, err
	}
	return out, nil
}

func appendResolvedPaths(s *Server, raw []string, asPrefix bool, out *local.PathAllow) error {
	if raw == nil {
		return nil
	}
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cands, err := s.cfg.AllowlistedCandidates(p)
		if err != nil {
			return err
		}
		switch len(cands) {
		case 0:
			return fmt.Errorf("path %q is outside allowed roots", p)
		case 1:
			if asPrefix {
				out.Prefixes = append(out.Prefixes, cands[0])
			} else {
				out.Exact = append(out.Exact, cands[0])
			}
		default:
			return fmt.Errorf("path %q is ambiguous; matches: %s", p, strings.Join(cands, ", "))
		}
	}
	return nil
}
