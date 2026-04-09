#!/usr/bin/env python3

import argparse
import csv
from collections import defaultdict
from pathlib import Path

import matplotlib.pyplot as plt


def load_csv(path: Path) -> list[dict]:
    with path.open(newline="", encoding="utf-8") as handle:
        return list(csv.DictReader(handle))


def trial_rows(rows: list[dict]) -> list[dict]:
    return [row for row in rows if row.get("row_type") == "trial"]


def summary_map(rows: list[dict]) -> dict[str, float]:
    summary = {}
    for row in rows:
        if row.get("row_type") != "summary":
            continue
        key = row.get("summary_metric") or ""
        value = row.get("summary_value") or ""
        if not key or value == "":
            continue
        summary[key] = float(value)
    return summary


def aggregate_nonstream_by_prompt(rows: list[dict]) -> dict[str, list[float]]:
    grouped = defaultdict(lambda: {"direct": [], "proxy": []})
    for row in trial_rows(rows):
        prompt_words = int(row["prompt_word_count"])
        grouped[prompt_words]["direct"].append(float(row["direct_elapsed_ms"]))
        grouped[prompt_words]["proxy"].append(float(row["proxy_elapsed_ms"]))

    prompt_words = sorted(grouped)
    return {
        "prompt_words": prompt_words,
        "direct": [sum(grouped[word]["direct"]) / len(grouped[word]["direct"]) for word in prompt_words],
        "proxy": [sum(grouped[word]["proxy"]) / len(grouped[word]["proxy"]) for word in prompt_words],
    }


def aggregate_stream_by_prompt(rows: list[dict]) -> dict[str, list[float]]:
    grouped = defaultdict(lambda: {"direct_first": [], "proxy_first": [], "direct_total": [], "proxy_total": []})
    for row in trial_rows(rows):
        prompt_words = int(row["prompt_word_count"])
        grouped[prompt_words]["direct_first"].append(float(row["direct_first_chunk_ms"]))
        grouped[prompt_words]["proxy_first"].append(float(row["proxy_first_chunk_ms"]))
        grouped[prompt_words]["direct_total"].append(float(row["direct_total_elapsed_ms"]))
        grouped[prompt_words]["proxy_total"].append(float(row["proxy_total_elapsed_ms"]))

    prompt_words = sorted(grouped)
    return {
        "prompt_words": prompt_words,
        "direct_first": [sum(grouped[word]["direct_first"]) / len(grouped[word]["direct_first"]) for word in prompt_words],
        "proxy_first": [sum(grouped[word]["proxy_first"]) / len(grouped[word]["proxy_first"]) for word in prompt_words],
        "direct_total": [sum(grouped[word]["direct_total"]) / len(grouped[word]["direct_total"]) for word in prompt_words],
        "proxy_total": [sum(grouped[word]["proxy_total"]) / len(grouped[word]["proxy_total"]) for word in prompt_words],
    }


def latest_matching_csv(results_dir: Path, prefix: str) -> Path:
    matches = sorted(results_dir.glob(prefix))
    if not matches:
        raise FileNotFoundError(f"no files matching {prefix} in {results_dir}")
    return matches[-1]


def plot_prompt_curves(nonstream_rows: list[dict], stream_rows: list[dict], output_path: Path) -> None:
    nonstream = aggregate_nonstream_by_prompt(nonstream_rows)
    stream = aggregate_stream_by_prompt(stream_rows)

    plt.style.use("seaborn-v0_8-whitegrid")
    fig, axes = plt.subplots(2, 1, figsize=(12, 11), sharex=True)
    fig.patch.set_facecolor("#0f1318")

    for axis in axes:
        axis.set_facecolor("#151b22")
        axis.tick_params(colors="#d9e2ec")
        axis.xaxis.label.set_color("#d9e2ec")
        axis.yaxis.label.set_color("#d9e2ec")
        axis.title.set_color("#f0f4f8")
        for spine in axis.spines.values():
            spine.set_color("#3b4754")

    axes[0].plot(nonstream["prompt_words"], nonstream["direct"], color="#7cb7ff", linewidth=2.4, label="Direct Ollama")
    axes[0].plot(nonstream["prompt_words"], nonstream["proxy"], color="#ff9b71", linewidth=2.4, label="Via LlamaSitter")
    axes[0].set_title("Non-Streaming Completion Time by Prompt Size", fontsize=16, pad=12)
    axes[0].set_ylabel("Completion Time (ms)", fontsize=12)
    axes[0].legend(frameon=False, loc="upper left", fontsize=11)

    axes[1].plot(stream["prompt_words"], stream["direct_first"], color="#7cb7ff", linewidth=2.0, linestyle="--", label="Direct first chunk")
    axes[1].plot(stream["prompt_words"], stream["proxy_first"], color="#ff9b71", linewidth=2.0, linestyle="--", label="Proxy first chunk")
    axes[1].plot(stream["prompt_words"], stream["direct_total"], color="#44d7b6", linewidth=2.3, label="Direct total completion")
    axes[1].plot(stream["prompt_words"], stream["proxy_total"], color="#ffd166", linewidth=2.3, label="Proxy total completion")
    axes[1].set_title("Streaming First-Chunk and Completion Time by Prompt Size", fontsize=16, pad=12)
    axes[1].set_ylabel("Latency (ms)", fontsize=12)
    axes[1].set_xlabel("Prompt Length (words)", fontsize=12)
    axes[1].legend(frameon=False, loc="upper left", ncol=2, fontsize=10)

    fig.suptitle("LlamaSitter vs Direct Ollama Latency Benchmarks", fontsize=18, color="#f8fbff", y=0.98)
    fig.tight_layout(rect=(0, 0, 1, 0.965))
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, dpi=180, facecolor=fig.get_facecolor(), bbox_inches="tight")
    plt.close(fig)


def plot_summary_overhead(nonstream_rows: list[dict], stream_rows: list[dict], output_path: Path) -> None:
    nonstream_summary = summary_map(nonstream_rows)
    stream_summary = summary_map(stream_rows)

    labels = [
        "Non-stream\nmean total",
        "Non-stream\nmedian total",
        "Streaming\nmean first chunk",
        "Streaming\nmedian first chunk",
        "Streaming\nmean total",
        "Streaming\nmedian total",
    ]
    values = [
        nonstream_summary["mean_elapsed_delta_ms"],
        nonstream_summary["median_elapsed_delta_ms"],
        stream_summary["mean_first_chunk_delta_ms"],
        stream_summary["median_first_chunk_delta_ms"],
        stream_summary["mean_total_elapsed_delta_ms"],
        stream_summary["median_total_elapsed_delta_ms"],
    ]

    plt.style.use("seaborn-v0_8-whitegrid")
    fig, ax = plt.subplots(figsize=(12, 7))
    fig.patch.set_facecolor("#0f1318")
    ax.set_facecolor("#151b22")
    ax.tick_params(colors="#d9e2ec")
    ax.xaxis.label.set_color("#d9e2ec")
    ax.yaxis.label.set_color("#d9e2ec")
    ax.title.set_color("#f0f4f8")
    for spine in ax.spines.values():
        spine.set_color("#3b4754")

    colors = ["#ff9b71" if value > 0 else "#44d7b6" for value in values]
    bars = ax.bar(labels, values, color=colors, width=0.68)
    ax.axhline(0, color="#d9e2ec", linewidth=1.2)
    ax.set_ylabel("Proxy Overhead vs Direct (ms)", fontsize=12)
    ax.set_title("Balanced Crossover Results: LlamaSitter Adds Only Small Latency", fontsize=17, pad=14)

    for bar, value in zip(bars, values, strict=True):
        va = "bottom" if value >= 0 else "top"
        offset = 4 if value >= 0 else -4
        ax.text(
            bar.get_x() + bar.get_width() / 2,
            value + offset,
            f"{value:+.1f} ms",
            ha="center",
            va=va,
            color="#f8fbff",
            fontsize=11,
            fontweight="bold",
        )

    ax.text(
        0.99,
        0.04,
        "Positive means the proxy was slower.\nNegative means the proxy was faster.",
        transform=ax.transAxes,
        ha="right",
        va="bottom",
        color="#a9b7c6",
        fontsize=10,
    )

    fig.tight_layout()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, dpi=180, facecolor=fig.get_facecolor(), bbox_inches="tight")
    plt.close(fig)


def main() -> int:
    parser = argparse.ArgumentParser(description="Render documentation-ready plots for LlamaSitter latency benchmarks.")
    parser.add_argument("--results-dir", default="benchmarks/results")
    parser.add_argument("--nonstream-csv", default="")
    parser.add_argument("--stream-csv", default="")
    parser.add_argument("--figures-dir", default="benchmarks/figures")
    args = parser.parse_args()

    results_dir = Path(args.results_dir)
    figures_dir = Path(args.figures_dir)

    nonstream_csv = Path(args.nonstream_csv) if args.nonstream_csv else latest_matching_csv(results_dir, "ollama_vs_llamasitter_crossover_latency_*.csv")
    stream_csv = Path(args.stream_csv) if args.stream_csv else latest_matching_csv(results_dir, "ollama_vs_llamasitter_streaming_crossover_latency_*.csv")

    nonstream_rows = load_csv(nonstream_csv)
    stream_rows = load_csv(stream_csv)

    curves_path = figures_dir / "llamasitter_latency_prompt_curves.png"
    summary_path = figures_dir / "llamasitter_latency_overhead_summary.png"

    plot_prompt_curves(nonstream_rows, stream_rows, curves_path)
    plot_summary_overhead(nonstream_rows, stream_rows, summary_path)

    print(f"Used non-stream CSV: {nonstream_csv}")
    print(f"Used stream CSV:     {stream_csv}")
    print(f"Wrote figure:        {curves_path}")
    print(f"Wrote figure:        {summary_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
