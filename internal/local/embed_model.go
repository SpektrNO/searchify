package local

import (
	"fmt"
	"log/slog"
)

// reconcileEmbedModelForWrite clears chunk_vectors when the configured model
// differs from the one stored in meta (dimensions must not be mixed).
// Returns true if existing vectors were deleted.
func (s *Service) reconcileEmbedModelForWrite() (cleared bool, err error) {
	want := s.cfg.EmbedModel
	if want == "" {
		return false, nil
	}
	stored, err := s.getMeta("embed_model")
	if err != nil {
		return false, err
	}
	if stored == "" || stored == want {
		return false, nil
	}
	count, err := s.VectorCount()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return false, s.setMeta("embed_model", want)
	}
	if _, err := s.db.Exec(`DELETE FROM chunk_vectors`); err != nil {
		return false, fmt.Errorf("clear vectors for embed model change: %w", err)
	}
	if err := s.setMeta("embed_model", want); err != nil {
		return false, err
	}
	slog.Warn("cleared chunk_vectors after SEARCHIFY_EMBED_MODEL change",
		"from", stored, "to", want, "deleted", count)
	return true, nil
}

// requireMatchingEmbedModel fails vector search when stored vectors were built
// with a different model than the current config.
func (s *Service) requireMatchingEmbedModel() error {
	want := s.cfg.EmbedModel
	if want == "" {
		return nil
	}
	stored, err := s.getMeta("embed_model")
	if err != nil {
		return err
	}
	if stored == "" || stored == want {
		return nil
	}
	count, err := s.VectorCount()
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	return fmt.Errorf(
		"index embed_model=%q but SEARCHIFY_EMBED_MODEL=%q; run `searchify embed --force` (or re-index) to rebuild vectors",
		stored, want,
	)
}
