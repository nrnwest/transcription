#!/usr/bin/env python3
"""Speaker diarization via pyannote/speaker-diarization-3.1.

Embedded into the transcription binary and materialized to a temp file at
runtime. Prints segments in the line format shared with the Go parser:

    0.318 -- 5.121 speaker_00

All diagnostics go to stderr; a non-zero exit code signals failure.
The HF_TOKEN environment variable is needed only for the first model
download; afterwards the pipeline loads from the local HuggingFace cache.
"""
import argparse
import os
import sys


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("wav")
    ap.add_argument("--num-speakers", type=int, default=0)
    ap.add_argument("--model-dir", default="")
    args = ap.parse_args()

    # Some pyannote ops lack MPS kernels; fall back to CPU transparently.
    os.environ.setdefault("PYTORCH_ENABLE_MPS_FALLBACK", "1")

    try:
        import torch
        from pyannote.audio import Pipeline
    except ImportError as exc:
        print(
            f"pyannote.audio is not installed: {exc} — "
            'see the "Speaker diarization" section in the README',
            file=sys.stderr,
        )
        return 3

    token = os.environ.get("HF_TOKEN")

    # A local model directory (config.yaml + weights) takes priority: it needs
    # no HuggingFace account, token, or network at all. Otherwise fall back to
    # the hub: the current community-1 pipeline (pyannote.audio 4.x), then the
    # older 3.1 (pyannote.audio 3.x installs).
    if args.model_dir:
        candidates = [args.model_dir]
    else:
        candidates = [
            "pyannote/speaker-diarization-community-1",
            "pyannote/speaker-diarization-3.1",
        ]

    def load(name):
        # pyannote.audio 4.x renamed use_auth_token= to token=.
        try:
            return Pipeline.from_pretrained(name, token=token)
        except TypeError:
            return Pipeline.from_pretrained(name, use_auth_token=token)

    pipeline = None
    for name in candidates:
        try:
            pipeline = load(name)
        except Exception as exc:  # gated model, no cache, no network, ...
            print(f"failed to load {name}: {exc}", file=sys.stderr)
            pipeline = None
        if pipeline is not None:
            break
    if pipeline is None:
        print(
            "failed to load a pyannote pipeline — accept the model conditions "
            "on huggingface.co (see the README for the exact pages) and set "
            "HF_TOKEN for the first run",
            file=sys.stderr,
        )
        return 4

    if torch.cuda.is_available():
        pipeline.to(torch.device("cuda"))
    elif torch.backends.mps.is_available():
        pipeline.to(torch.device("mps"))

    kwargs = {}
    if args.num_speakers > 0:
        kwargs["num_speakers"] = args.num_speakers

    result = pipeline(args.wav, **kwargs)
    # pyannote.audio 4.x wraps the Annotation in a DiarizeOutput; 3.x returns
    # the Annotation directly.
    annotation = getattr(result, "speaker_diarization", result)

    # Remap pyannote's "SPEAKER_00" labels to ints by first appearance so the
    # output matches the Go parser regex exactly.
    ids = {}
    for turn, _, speaker in annotation.itertracks(yield_label=True):
        idx = ids.setdefault(speaker, len(ids))
        print(f"{turn.start:.3f} -- {turn.end:.3f} speaker_{idx:02d}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
