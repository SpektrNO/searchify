package local

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	kjarni "github.com/olafurjohannsson/kjarni-go"
)

func encodeFloat32(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeFloat32(buf []byte) ([]float32, error) {
	if len(buf)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding length %d", len(buf))
	}
	out := make([]float32, len(buf)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return out, nil
}

func (s *Service) upsertChunkVector(chunkID string, embedding []float32) error {
	_, err := s.db.Exec(
		`INSERT INTO chunk_vectors(chunk_id, embedding) VALUES (?, ?)
		 ON CONFLICT(chunk_id) DO UPDATE SET embedding = excluded.embedding`,
		chunkID, encodeFloat32(embedding),
	)
	return err
}

func (s *Service) VectorCount() (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM chunk_vectors`).Scan(&count)
	return count, err
}

type vectorHit struct {
	chunkID string
	score   float32
}

func topKByCosine(query []float32, vectors map[string][]float32, k int) []vectorHit {
	if len(vectors) == 0 || k <= 0 {
		return nil
	}

	hits := make([]vectorHit, 0, len(vectors))
	for id, vec := range vectors {
		if len(vec) != len(query) {
			continue
		}
		hits = append(hits, vectorHit{
			chunkID: id,
			score:   kjarni.CosineSimilarity(query, vec),
		})
	}

	sort.Slice(hits, func(i, j int) bool {
		return hits[i].score > hits[j].score
	})
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}

func (s *Service) loadAllChunkVectors() (map[string][]float32, error) {
	return s.loadChunkVectors(nil)
}

// loadChunkVectors loads embeddings, optionally constrained to PathAllow before ranking.
func (s *Service) loadChunkVectors(allow *PathAllow) (map[string][]float32, error) {
	if allow != nil && allow.emptyConstraint() {
		return map[string][]float32{}, nil
	}

	query := `SELECT v.chunk_id, v.embedding FROM chunk_vectors v`
	var args []any
	if allow != nil {
		clause, pathArgs := pathAllowSQL("c", allow)
		query = `SELECT v.chunk_id, v.embedding
			FROM chunk_vectors v
			JOIN chunks_fts c ON c.id = v.chunk_id
			WHERE 1=1` + clause
		args = pathArgs
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vectors := make(map[string][]float32)
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		vec, err := decodeFloat32(blob)
		if err != nil {
			return nil, fmt.Errorf("decode %q: %w", id, err)
		}
		vectors[id] = vec
	}
	return vectors, rows.Err()
}
