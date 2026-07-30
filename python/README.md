# Offline embedding generation

Scoped strictly to offline batch embedding generation, per the project's
stack mandate - Go owns the crawler, index builder, and query service
(including the HNSW index itself); Python's only job here is producing the
embedding vectors ahead of time. See `docs/design/25-embeddings-format.md`.

## Setup

```bash
cd python
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

## Usage

```bash
python3 generate_embeddings.py input.jsonl output.bin
```

`input.jsonl`: one `{"id": "...", "text": "..."}` object per line. `id`
must be exactly a 64-character `extract.Passage.ID()` value.

`output.bin`: the binary embeddings file (format `TTE1`), readable via
`internal/embeddings.Open` in Go.
