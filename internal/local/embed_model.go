package local

import (
	"fmt"
	"log/slog"

	"github.com/spektr/searchify/internal/config"
)

func (s *Service) wantEmbedEngine() string {
	return string(s.cfg.EffectiveEmbedEngine())
}

func (s *Service) setEmbedIdentityMeta() error {
	if err := s.setMeta("embed_model", s.cfg.EmbedModel); err != nil {
		return err
	}
	return s.setMeta("embed_engine", s.wantEmbedEngine())
}

// reconcileEmbedModelForWrite clears chunk_vectors when the configured engine
// or model differs from meta (vector spaces must not be mixed).
// Returns true if existing vectors were deleted.
func (s *Service) reconcileEmbedModelForWrite() (cleared bool, err error) {
	wantModel := s.cfg.EmbedModel
	wantEngine := s.wantEmbedEngine()
	if wantModel == "" {
		return false, nil
	}
	storedModel, err := s.getMeta("embed_model")
	if err != nil {
		return false, err
	}
	storedEngine, err := s.getMeta("embed_engine")
	if err != nil {
		return false, err
	}
	// Legacy indexes only have embed_model; treat missing engine as kjarni.
	if storedEngine == "" {
		storedEngine = string(config.EmbedEngineKjarni)
	}
	same := storedModel == wantModel && storedEngine == wantEngine
	if storedModel == "" {
		if err := s.setEmbedIdentityMeta(); err != nil {
			return false, err
		}
		return false, nil
	}
	if same {
		return false, nil
	}
	count, err := s.VectorCount()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return false, s.setEmbedIdentityMeta()
	}
	if _, err := s.db.Exec(`DELETE FROM chunk_vectors`); err != nil {
		return false, fmt.Errorf("clear vectors for embed engine/model change: %w", err)
	}
	if err := s.setEmbedIdentityMeta(); err != nil {
		return false, err
	}
	slog.Warn("cleared chunk_vectors after embed engine/model change",
		"from_engine", storedEngine, "to_engine", wantEngine,
		"from_model", storedModel, "to_model", wantModel, "deleted", count)
	return true, nil
}

// requireMatchingEmbedModel fails vector search when stored vectors were built
// with a different engine/model than the current config.
func (s *Service) requireMatchingEmbedModel() error {
	wantModel := s.cfg.EmbedModel
	wantEngine := s.wantEmbedEngine()
	if wantModel == "" {
		return nil
	}
	storedModel, err := s.getMeta("embed_model")
	if err != nil {
		return err
	}
	storedEngine, err := s.getMeta("embed_engine")
	if err != nil {
		return err
	}
	if storedEngine == "" {
		storedEngine = string(config.EmbedEngineKjarni)
	}
	if storedModel == "" || (storedModel == wantModel && storedEngine == wantEngine) {
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
		"index embed_engine=%q embed_model=%q but config engine=%q model=%q; run `searchify embed --force` (or re-index) to rebuild vectors",
		storedEngine, storedModel, wantEngine, wantModel,
	)
}
