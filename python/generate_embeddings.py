#!/usr/bin/env python3
"""Generates passage embeddings and writes them in Tried & Told's binary
embeddings format (TTE1). See docs/design/25-embeddings-format.md.

Input: a JSONL file, one {"id": "...", "text": "..."} object per line.
"id" must be exactly a 64-character extract.Passage.ID() (a hex SHA-256
digest) - the same join key used by the dedup registry, deletion log, and
attribution registry throughout the rest of this project.

This script embeds PASSAGES, not queries. BGE models use asymmetric
encoding: queries get an instruction prefix ("Represent this sentence for
searching relevant passages: "), passages get none. Encoding a query
correctly is the query-service's job at search time, not this script's.

Usage:
    python3 generate_embeddings.py input.jsonl output.bin
"""
import json
import struct
import sys
import time
import zlib

from sentence_transformers import SentenceTransformer

MODEL_NAME = "BAAI/bge-small-en-v1.5"

# Identifies both the model AND our own usage convention (normalized,
# cosine-compared, no query instruction added to passages) in one string -
# bump this if either changes, so a consumer can check it before trusting
# the file's contents. Not the same thing as the model's own HuggingFace
# revision; this is our own "did anything relevant to us change" marker.
VERSION = "bge-small-en-v1.5-cosine-normalized-v1"

PASSAGE_ID_SIZE = 64
MAGIC = b"TTE1"

# "<" = little-endian, standard sizes, no padding - matches Go's
# encoding/binary.Write field-by-field serialization exactly (verified,
# not assumed, via docs/design/25-embeddings-format.md).
HEADER_FORMAT = "<4sIIqQQQQ"
HEADER_SIZE = struct.calcsize(HEADER_FORMAT)


def load_passages(path):
    ids, texts = [], []
    with open(path, "r", encoding="utf-8") as f:
        for line_num, line in enumerate(f, start=1):
            line = line.strip()
            if not line:
                continue
            obj = json.loads(line)
            pid = obj["id"]
            if len(pid) != PASSAGE_ID_SIZE:
                raise ValueError(
                    f"line {line_num}: passage id {pid!r} is {len(pid)} chars, "
                    f"want exactly {PASSAGE_ID_SIZE}"
                )
            ids.append(pid)
            texts.append(obj["text"])
    return ids, texts


def main():
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} input.jsonl output.bin", file=sys.stderr)
        sys.exit(1)
    input_path, output_path = sys.argv[1], sys.argv[2]

    ids, texts = load_passages(input_path)
    if not ids:
        print("no passages to embed", file=sys.stderr)
        sys.exit(1)

    model = SentenceTransformer(MODEL_NAME)
    embeddings = model.encode(texts, normalize_embeddings=True)
    dim = embeddings.shape[1]

    version_bytes = VERSION.encode("utf-8")
    version_off = HEADER_SIZE
    version_len = len(version_bytes)
    records_off = version_off + version_len
    records_len = len(ids) * (PASSAGE_ID_SIZE + dim * 4)

    with open(output_path, "wb") as f:
        f.write(struct.pack(
            HEADER_FORMAT,
            MAGIC,
            dim,
            len(ids),
            int(time.time()),
            version_off,
            version_len,
            records_off,
            records_len,
        ))
        f.write(version_bytes)

        for pid, vec in zip(ids, embeddings):
            f.write(pid.encode("ascii"))
            f.write(struct.pack(f"<{dim}f", *vec.tolist()))

    with open(output_path, "rb") as f:
        checksum = zlib.crc32(f.read()) & 0xFFFFFFFF
    with open(output_path, "ab") as f:
        f.write(struct.pack("<I", checksum))

    print(f"wrote {len(ids)} embeddings ({dim} dims) to {output_path}")


if __name__ == "__main__":
    main()
