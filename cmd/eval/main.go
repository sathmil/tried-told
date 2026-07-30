// Command eval runs the judged query set against the live BM25 search and
// reports Precision@10 and MRR, per docs/design/05-eval-milestone1.md.
package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"triedandtold/internal/bm25"
	"triedandtold/internal/corpus"
	"triedandtold/internal/eval"
	"triedandtold/internal/index"
)

const k = 10

func main() {
	docs, _, err := corpus.LoadJSONL("data/synthetic/experiences.jsonl")
	if err != nil {
		log.Fatalf("loading corpus: %v", err)
	}
	idx := index.BuildIndex(docs)

	queries, err := eval.LoadQueries("data/eval/queries.json")
	if err != nil {
		log.Fatalf("loading queries: %v", err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "QUERY\tP@%d\tRR\n", k)

	var sumPrecision, sumRR float64
	for _, q := range queries {
		hits := bm25.Search(bm25.WrapInMemory(idx), q.Text, bm25.DefaultParams)
		ranked := make([]int, len(hits))
		for i, h := range hits {
			ranked[i] = h.DocID
		}

		p := eval.PrecisionAtK(ranked, q, k)
		rr := eval.ReciprocalRank(ranked, q)
		sumPrecision += p
		sumRR += rr

		fmt.Fprintf(tw, "%s\t%.2f\t%.2f\n", truncate(q.Text, 60), p, rr)
	}
	tw.Flush()

	n := float64(len(queries))
	fmt.Printf("\nMean Precision@%d: %.3f\n", k, sumPrecision/n)
	fmt.Printf("MRR: %.3f\n", sumRR/n)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
