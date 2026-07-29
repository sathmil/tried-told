// Package eval implements evaluation metrics and the judged-query harness,
// per docs/design/05-eval-milestone1.md.
package eval

import (
	"encoding/json"
	"os"
)

// Judgment records a graded relevance label for one document: 0 = irrelevant,
// 1 = somewhat relevant, 2 = highly relevant.
type Judgment struct {
	DocID     int `json:"doc_id"`
	Relevance int `json:"relevance"`
}

// Query is one judged evaluation query. Judgments are sparse: any DocID not
// listed is implicitly irrelevant (0) - see docs/design/05-eval-milestone1.md
// for why exhaustive judging doesn't scale.
type Query struct {
	Text      string     `json:"query"`
	Note      string     `json:"note,omitempty"`
	Judgments []Judgment `json:"judgments"`
}

// Relevance returns this query's judged relevance for docID, defaulting to 0
// (irrelevant) if the document wasn't judged.
func (q Query) Relevance(docID int) int {
	for _, j := range q.Judgments {
		if j.DocID == docID {
			return j.Relevance
		}
	}
	return 0
}

// LoadQueries reads a JSON array of judged queries from path.
func LoadQueries(path string) ([]Query, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var queries []Query
	if err := json.Unmarshal(data, &queries); err != nil {
		return nil, err
	}
	return queries, nil
}
