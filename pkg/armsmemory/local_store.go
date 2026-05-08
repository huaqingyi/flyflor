//go:build !mipsle && !netbsd && !(freebsd && arm)

package armsmemory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/hnsw"
	"github.com/dhconnelly/rtreego"
	_ "modernc.org/sqlite"
)

const palaceDimensions = 8

type LocalStore struct {
	mu    sync.RWMutex
	cfg   Config
	db    *sql.DB
	graph *hnsw.Graph[string]
	tree  *rtreego.Rtree
	docs  map[string]localMethodologyDoc
}

type localMethodologyDoc struct {
	Methodology
	Vector []float32
	Coords []float64
}

type palaceSpatial struct {
	id     string
	bounds rtreego.Rect
}

func (s palaceSpatial) Bounds() rtreego.Rect {
	return s.bounds
}

func NewLocalStore(cfg Config) (*LocalStore, error) {
	cfg.normalize()
	path, err := expandPath(cfg.Path)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("arms local path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("arms local mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("arms local open sqlite: %w", err)
	}
	store := &LocalStore{
		cfg:  cfg,
		db:   db,
		docs: map[string]localMethodologyDoc{},
	}
	if err := store.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.rebuildIndexes(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *LocalStore) UpsertMethodologies(ctx context.Context, methodologies []Methodology) error {
	if len(methodologies) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, methodology := range methodologies {
		methodology.Namespace = firstNonEmpty(methodology.Namespace, s.cfg.MethodologyNamespace)
		methodology.Source = firstNonEmpty(methodology.Source, "flyflor.reflection")
		methodology.SchemaVersion = methodologySchemaVersion
		if methodology.ID == "" {
			methodology.ID = stableMethodologyID(s.cfg.SpaceID, methodology.Namespace, methodology.Text)
		}
		if methodology.CreatedAt == "" {
			methodology.CreatedAt = now
		}
		vector := embedText(methodologySearchText(methodology), s.cfg.Dimensions)
		coords := palaceCoords(methodology)
		tagsJSON, _ := json.Marshal(methodology.Tags)
		vectorJSON, _ := json.Marshal(vector)
		coordsJSON, _ := json.Marshal(coords)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO arms_methodologies (
  id, space_id, namespace, title, text, situation, method, avoid, next_time_hint,
  tags_json, session_key, source, schema_version, vector_json, coords_json,
  created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  space_id=excluded.space_id,
  namespace=excluded.namespace,
  title=excluded.title,
  text=excluded.text,
  situation=excluded.situation,
  method=excluded.method,
  avoid=excluded.avoid,
  next_time_hint=excluded.next_time_hint,
  tags_json=excluded.tags_json,
  session_key=excluded.session_key,
  source=excluded.source,
  schema_version=excluded.schema_version,
  vector_json=excluded.vector_json,
  coords_json=excluded.coords_json,
  updated_at=excluded.updated_at
`, methodology.ID, s.cfg.SpaceID, methodology.Namespace, methodology.Title, methodology.Text,
			methodology.Situation, methodology.Method, methodology.Avoid, methodology.NextTimeHint,
			string(tagsJSON), methodology.SessionKey, methodology.Source, methodology.SchemaVersion,
			string(vectorJSON), string(coordsJSON), methodology.CreatedAt, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.rebuildIndexesLocked(ctx)
}

func (s *LocalStore) SearchMethodologies(ctx context.Context, query string, limit int, minScore float64) ([]SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultTopK
	}
	if minScore <= 0 {
		minScore = defaultMinScore
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.docs) == 0 {
		return nil, nil
	}

	candidates := map[string]float64{}
	queryVector := embedText(query, s.cfg.Dimensions)
	for _, node := range s.graph.Search(queryVector, limit*4) {
		distance := float64(hnsw.CosineDistance(queryVector, node.Value))
		candidates[node.Key] = math.Max(candidates[node.Key], clampScore(1-distance))
	}
	for _, spatial := range s.tree.NearestNeighbors(limit*4, rtreego.Point(queryCoords(query))) {
		obj, ok := spatial.(palaceSpatial)
		if !ok {
			continue
		}
		doc, ok := s.docs[obj.id]
		if !ok {
			continue
		}
		spatialScore := 1 / (1 + euclidean(queryCoords(query), doc.Coords))
		candidates[obj.id] = math.Max(candidates[obj.id], spatialScore*0.88)
	}

	hits := make([]SearchHit, 0, len(candidates))
	for id, score := range candidates {
		if score < minScore {
			continue
		}
		doc, ok := s.docs[id]
		if !ok {
			continue
		}
		hits = append(hits, SearchHit{
			Title:        doc.Title,
			Text:         doc.Text,
			Situation:    doc.Situation,
			Method:       doc.Method,
			Avoid:        doc.Avoid,
			NextTimeHint: doc.NextTimeHint,
			Tags:         append([]string(nil), doc.Tags...),
			SessionKey:   doc.SessionKey,
			Score:        score,
			CreatedAt:    doc.CreatedAt,
		})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (s *LocalStore) Status(ctx context.Context) (Status, error) {
	s.mu.RLock()
	count := len(s.docs)
	s.mu.RUnlock()
	return Status{
		Enabled:   true,
		Ready:     true,
		Driver:    "local",
		Path:      s.cfg.Path,
		SpaceID:   s.cfg.SpaceID,
		Namespace: s.cfg.MethodologyNamespace,
		Count:     count,
	}, nil
}

func (s *LocalStore) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS arms_methodologies (
  id TEXT PRIMARY KEY,
  space_id TEXT NOT NULL,
  namespace TEXT NOT NULL,
  title TEXT NOT NULL,
  text TEXT NOT NULL,
  situation TEXT,
  method TEXT,
  avoid TEXT,
  next_time_hint TEXT,
  tags_json TEXT,
  session_key TEXT,
  source TEXT NOT NULL,
  schema_version INTEGER NOT NULL,
  vector_json TEXT NOT NULL,
  coords_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  hit_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_arms_methodologies_space_namespace
  ON arms_methodologies(space_id, namespace);
`)
	if err != nil {
		return fmt.Errorf("arms local schema: %w", err)
	}
	return nil
}

func (s *LocalStore) rebuildIndexes(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rebuildIndexesLocked(ctx)
}

func (s *LocalStore) rebuildIndexesLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, text, situation, method, avoid, next_time_hint, tags_json,
       session_key, source, schema_version, vector_json, coords_json, created_at
FROM arms_methodologies
WHERE space_id = ? AND namespace = ?
`, s.cfg.SpaceID, s.cfg.MethodologyNamespace)
	if err != nil {
		return err
	}
	defer rows.Close()

	graph := hnsw.NewGraph[string]()
	graph.Distance = hnsw.CosineDistance
	graph.M = 16
	graph.Ml = 0.25
	graph.EfSearch = 24
	tree := rtreego.NewTree(palaceDimensions, 4, 8)
	docs := map[string]localMethodologyDoc{}

	for rows.Next() {
		var doc localMethodologyDoc
		var tagsJSON, vectorJSON, coordsJSON string
		if err := rows.Scan(
			&doc.ID, &doc.Title, &doc.Text, &doc.Situation, &doc.Method, &doc.Avoid,
			&doc.NextTimeHint, &tagsJSON, &doc.SessionKey, &doc.Source, &doc.SchemaVersion,
			&vectorJSON, &coordsJSON, &doc.CreatedAt,
		); err != nil {
			return err
		}
		_ = json.Unmarshal([]byte(tagsJSON), &doc.Tags)
		_ = json.Unmarshal([]byte(vectorJSON), &doc.Vector)
		_ = json.Unmarshal([]byte(coordsJSON), &doc.Coords)
		if len(doc.Vector) != s.cfg.Dimensions {
			doc.Vector = embedText(methodologySearchText(doc.Methodology), s.cfg.Dimensions)
		}
		if len(doc.Coords) != palaceDimensions {
			doc.Coords = palaceCoords(doc.Methodology)
		}
		docs[doc.ID] = doc
		graph.Add(hnsw.MakeNode(doc.ID, hnsw.Vector(doc.Vector)))
		if spatial, ok := newPalaceSpatial(doc.ID, doc.Coords); ok {
			tree.Insert(spatial)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.graph = graph
	s.tree = tree
	s.docs = docs
	return nil
}

func newPalaceSpatial(id string, coords []float64) (palaceSpatial, bool) {
	if len(coords) != palaceDimensions {
		return palaceSpatial{}, false
	}
	lengths := make([]float64, palaceDimensions)
	for i := range lengths {
		lengths[i] = 0.0001
	}
	rect, err := rtreego.NewRect(rtreego.Point(coords), lengths)
	if err != nil {
		return palaceSpatial{}, false
	}
	return palaceSpatial{id: id, bounds: rect}, true
}

func methodologySearchText(m Methodology) string {
	return strings.Join([]string{m.Title, m.Text, m.Situation, m.Method, m.Avoid, m.NextTimeHint, strings.Join(m.Tags, " ")}, "\n")
}

func expandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func euclidean(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float64
	for i := 0; i < n; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

func clampScore(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
