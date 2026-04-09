#!/usr/bin/env python3

import argparse
import csv
import json
import statistics
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path


DEFAULT_DIRECT_ENDPOINT = "http://127.0.0.1:11434/api/chat"
DEFAULT_PROXY_ENDPOINT = "http://127.0.0.1:11435/api/chat"
DEFAULT_MODEL = "qwen3-vl:8b"
PAIR_ORDER_DIRECT_THEN_PROXY = "direct-then-proxy"
PAIR_ORDER_PROXY_THEN_DIRECT = "proxy-then-direct"


def iso_now() -> str:
    return datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def make_filler(word_count: int) -> str:
    lexicon = [
        "latency",
        "benchmark",
        "proxy",
        "ollama",
        "observability",
        "request",
        "response",
        "token",
        "local",
        "measurement",
    ]
    words = [lexicon[index % len(lexicon)] for index in range(word_count)]
    return " ".join(words)


def make_messages(word_count: int) -> list[dict[str, str]]:
    filler = make_filler(word_count)
    return [
        {
            "role": "system",
            "content": "Reply with exactly the single word OK. Do not explain anything.",
        },
        {
            "role": "user",
            "content": (
                "Read the filler text below and then reply with exactly OK.\n\n"
                f"{filler}\n\n"
                "Reply with exactly OK."
            ),
        },
    ]


def request_payload(model: str, word_count: int) -> dict:
    return {
        "model": model,
        "messages": make_messages(word_count),
        "stream": False,
        "keep_alive": "30m",
        "options": {
            "temperature": 0,
            "seed": 42,
            "num_predict": 8,
            "top_k": 1,
        },
    }


def send_chat(endpoint: str, payload: dict, extra_headers: dict[str, str] | None = None, timeout_seconds: float = 600.0) -> dict:
    body = json.dumps(payload).encode("utf-8")
    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json",
    }
    if extra_headers:
        headers.update(extra_headers)

    request = urllib.request.Request(endpoint, data=body, headers=headers, method="POST")
    started = time.perf_counter_ns()

    try:
        with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
            raw = response.read()
            elapsed_ms = (time.perf_counter_ns() - started) / 1_000_000
            parsed = json.loads(raw.decode("utf-8"))
            return {
                "ok": True,
                "http_status": response.status,
                "elapsed_ms": round(elapsed_ms, 3),
                "message_content": (((parsed.get("message") or {}).get("content")) or "").strip(),
                "prompt_tokens": int(parsed.get("prompt_eval_count") or 0),
                "output_tokens": int(parsed.get("eval_count") or 0),
                "total_tokens": int((parsed.get("prompt_eval_count") or 0) + (parsed.get("eval_count") or 0)),
                "reported_total_duration_ms": int((parsed.get("total_duration") or 0) / 1_000_000),
                "reported_prompt_eval_ms": int((parsed.get("prompt_eval_duration") or 0) / 1_000_000),
                "reported_eval_ms": int((parsed.get("eval_duration") or 0) / 1_000_000),
                "response_chars": len((((parsed.get("message") or {}).get("content")) or "")),
                "error": "",
            }
    except urllib.error.HTTPError as error:
        body_text = error.read().decode("utf-8", errors="replace")
        elapsed_ms = (time.perf_counter_ns() - started) / 1_000_000
        return {
            "ok": False,
            "http_status": error.code,
            "elapsed_ms": round(elapsed_ms, 3),
            "message_content": "",
            "prompt_tokens": 0,
            "output_tokens": 0,
            "total_tokens": 0,
            "reported_total_duration_ms": 0,
            "reported_prompt_eval_ms": 0,
            "reported_eval_ms": 0,
            "response_chars": 0,
            "error": body_text.strip(),
        }
    except Exception as error:  # noqa: BLE001
        elapsed_ms = (time.perf_counter_ns() - started) / 1_000_000
        return {
            "ok": False,
            "http_status": 0,
            "elapsed_ms": round(elapsed_ms, 3),
            "message_content": "",
            "prompt_tokens": 0,
            "output_tokens": 0,
            "total_tokens": 0,
            "reported_total_duration_ms": 0,
            "reported_prompt_eval_ms": 0,
            "reported_eval_ms": 0,
            "response_chars": 0,
            "error": str(error),
        }


def build_proxy_headers(session_id: str) -> dict[str, str]:
    return {
        "X-LlamaSitter-Client-Type": "benchmark",
        "X-LlamaSitter-Client-Instance": "latency-script",
        "X-LlamaSitter-Agent-Name": "direct-vs-proxy",
        "X-LlamaSitter-Session-Id": session_id,
    }


def make_summary_row(metric: str, value: float) -> dict:
    return {
        "row_type": "summary",
        "design": "",
        "sequence_index": "",
        "trial_index": "",
        "prompt_word_count": "",
        "prompt_char_count": "",
        "pair_order": "",
        "first_endpoint": "",
        "second_endpoint": "",
        "direct_position": "",
        "proxy_position": "",
        "model": "",
        "direct_http_status": "",
        "direct_elapsed_ms": "",
        "direct_prompt_tokens": "",
        "direct_output_tokens": "",
        "direct_total_tokens": "",
        "direct_reported_total_duration_ms": "",
        "direct_reported_prompt_eval_ms": "",
        "direct_reported_eval_ms": "",
        "proxy_http_status": "",
        "proxy_elapsed_ms": "",
        "proxy_prompt_tokens": "",
        "proxy_output_tokens": "",
        "proxy_total_tokens": "",
        "proxy_reported_total_duration_ms": "",
        "proxy_reported_prompt_eval_ms": "",
        "proxy_reported_eval_ms": "",
        "elapsed_delta_ms": "",
        "elapsed_ratio": "",
        "response_match": "",
        "error": "",
        "summary_metric": metric,
        "summary_value": f"{value:.3f}",
    }


def summary_rows(rows: list[dict]) -> list[dict]:
    successful = [row for row in rows if row["direct_http_status"] == 200 and row["proxy_http_status"] == 200]
    if not successful:
        return []

    direct_values = [row["direct_elapsed_ms"] for row in successful]
    proxy_values = [row["proxy_elapsed_ms"] for row in successful]
    delta_values = [row["elapsed_delta_ms"] for row in successful]

    rows_out = [
        make_summary_row("successful_pairs", float(len(successful))),
        make_summary_row("direct_mean_elapsed_ms", statistics.mean(direct_values)),
        make_summary_row("proxy_mean_elapsed_ms", statistics.mean(proxy_values)),
        make_summary_row("mean_elapsed_delta_ms", statistics.mean(delta_values)),
        make_summary_row("direct_median_elapsed_ms", statistics.median(direct_values)),
        make_summary_row("proxy_median_elapsed_ms", statistics.median(proxy_values)),
        make_summary_row("median_elapsed_delta_ms", statistics.median(delta_values)),
        make_summary_row("direct_max_elapsed_ms", max(direct_values)),
        make_summary_row("proxy_max_elapsed_ms", max(proxy_values)),
        make_summary_row("max_elapsed_delta_ms", max(delta_values)),
        make_summary_row("min_elapsed_delta_ms", min(delta_values)),
    ]

    for pair_order in (PAIR_ORDER_DIRECT_THEN_PROXY, PAIR_ORDER_PROXY_THEN_DIRECT):
        subset = [row for row in successful if row["pair_order"] == pair_order]
        if not subset:
            continue
        order_key = pair_order.replace("-", "_")
        rows_out.extend([
            make_summary_row(f"{order_key}_count", float(len(subset))),
            make_summary_row(f"{order_key}_direct_mean_elapsed_ms", statistics.mean(row["direct_elapsed_ms"] for row in subset)),
            make_summary_row(f"{order_key}_proxy_mean_elapsed_ms", statistics.mean(row["proxy_elapsed_ms"] for row in subset)),
            make_summary_row(f"{order_key}_mean_elapsed_delta_ms", statistics.mean(row["elapsed_delta_ms"] for row in subset)),
            make_summary_row(f"{order_key}_median_elapsed_delta_ms", statistics.median(row["elapsed_delta_ms"] for row in subset)),
        ])

    return rows_out


def run_pair(
    *,
    design: str,
    sequence_index: int,
    trial_index: int,
    word_count: int,
    payload: dict,
    pair_order: str,
    model: str,
    direct_endpoint: str,
    proxy_endpoint: str,
    session_id: str,
) -> dict:
    prompt_text = payload["messages"][1]["content"]
    proxy_headers = build_proxy_headers(session_id)

    if pair_order == PAIR_ORDER_DIRECT_THEN_PROXY:
        first_endpoint = "direct"
        second_endpoint = "proxy"
        direct_position = 1
        proxy_position = 2
        direct = send_chat(direct_endpoint, payload)
        proxy = send_chat(proxy_endpoint, payload, proxy_headers)
    elif pair_order == PAIR_ORDER_PROXY_THEN_DIRECT:
        first_endpoint = "proxy"
        second_endpoint = "direct"
        direct_position = 2
        proxy_position = 1
        proxy = send_chat(proxy_endpoint, payload, proxy_headers)
        direct = send_chat(direct_endpoint, payload)
    else:
        raise ValueError(f"unsupported pair order: {pair_order}")

    elapsed_delta_ms = round(proxy["elapsed_ms"] - direct["elapsed_ms"], 3)
    elapsed_ratio = round((proxy["elapsed_ms"] / direct["elapsed_ms"]) if direct["elapsed_ms"] else 0, 6)
    response_match = direct["message_content"] == proxy["message_content"]
    error = " | ".join(part for part in [direct["error"], proxy["error"]] if part)

    return {
        "row_type": "trial",
        "design": design,
        "sequence_index": sequence_index,
        "trial_index": trial_index,
        "prompt_word_count": word_count,
        "prompt_char_count": len(prompt_text),
        "pair_order": pair_order,
        "first_endpoint": first_endpoint,
        "second_endpoint": second_endpoint,
        "direct_position": direct_position,
        "proxy_position": proxy_position,
        "model": model,
        "direct_http_status": direct["http_status"],
        "direct_elapsed_ms": direct["elapsed_ms"],
        "direct_prompt_tokens": direct["prompt_tokens"],
        "direct_output_tokens": direct["output_tokens"],
        "direct_total_tokens": direct["total_tokens"],
        "direct_reported_total_duration_ms": direct["reported_total_duration_ms"],
        "direct_reported_prompt_eval_ms": direct["reported_prompt_eval_ms"],
        "direct_reported_eval_ms": direct["reported_eval_ms"],
        "proxy_http_status": proxy["http_status"],
        "proxy_elapsed_ms": proxy["elapsed_ms"],
        "proxy_prompt_tokens": proxy["prompt_tokens"],
        "proxy_output_tokens": proxy["output_tokens"],
        "proxy_total_tokens": proxy["total_tokens"],
        "proxy_reported_total_duration_ms": proxy["reported_total_duration_ms"],
        "proxy_reported_prompt_eval_ms": proxy["reported_prompt_eval_ms"],
        "proxy_reported_eval_ms": proxy["reported_eval_ms"],
        "elapsed_delta_ms": elapsed_delta_ms,
        "elapsed_ratio": elapsed_ratio,
        "response_match": response_match,
        "error": error,
        "summary_metric": "",
        "summary_value": "",
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Benchmark direct Ollama latency against the LlamaSitter proxy.")
    parser.add_argument("--model", default=DEFAULT_MODEL)
    parser.add_argument("--direct-endpoint", default=DEFAULT_DIRECT_ENDPOINT)
    parser.add_argument("--proxy-endpoint", default=DEFAULT_PROXY_ENDPOINT)
    parser.add_argument("--count", type=int, default=50)
    parser.add_argument("--min-words", type=int, default=10)
    parser.add_argument("--step-words", type=int, default=10)
    parser.add_argument("--design", choices=("sequential", "alternating", "crossover"), default="sequential")
    parser.add_argument("--output", default="")
    args = parser.parse_args()

    output_dir = Path("benchmarks/results")
    output_dir.mkdir(parents=True, exist_ok=True)

    if args.output:
        output_path = Path(args.output)
    else:
        output_path = output_dir / f"ollama_vs_llamasitter_{args.design}_latency_{iso_now()}.csv"

    session_id = f"benchmark-{iso_now()}"

    print(f"Benchmark model: {args.model}")
    print(f"Direct endpoint: {args.direct_endpoint}")
    print(f"Proxy endpoint:  {args.proxy_endpoint}")
    print(f"Design:          {args.design}")
    print(f"Output CSV:      {output_path}")
    print("Running warmup requests...")

    warmup_payload = request_payload(args.model, max(args.min_words, 20))
    warmup_direct = send_chat(args.direct_endpoint, warmup_payload)
    warmup_proxy = send_chat(args.proxy_endpoint, warmup_payload, build_proxy_headers(session_id))
    if not warmup_direct["ok"] or not warmup_proxy["ok"]:
        print("Warmup failed.", file=sys.stderr)
        print(f"Direct warmup: {warmup_direct}", file=sys.stderr)
        print(f"Proxy warmup:  {warmup_proxy}", file=sys.stderr)
        return 1

    rows: list[dict] = []
    sequence_index = 0
    for trial_index in range(1, args.count + 1):
        word_count = args.min_words + ((trial_index - 1) * args.step_words)
        payload = request_payload(args.model, word_count)
        if args.design == "alternating":
            pair_orders = [PAIR_ORDER_DIRECT_THEN_PROXY if trial_index % 2 == 1 else PAIR_ORDER_PROXY_THEN_DIRECT]
        elif args.design == "crossover":
            pair_orders = [PAIR_ORDER_DIRECT_THEN_PROXY, PAIR_ORDER_PROXY_THEN_DIRECT]
            if trial_index % 2 == 0:
                pair_orders.reverse()
        else:
            pair_orders = [PAIR_ORDER_DIRECT_THEN_PROXY]

        for pair_order in pair_orders:
            sequence_index += 1
            print(f"[{sequence_index:03d}] Prompt words={word_count} order={pair_order} ...")
            rows.append(run_pair(
                design=args.design,
                sequence_index=sequence_index,
                trial_index=trial_index,
                word_count=word_count,
                payload=payload,
                pair_order=pair_order,
                model=args.model,
                direct_endpoint=args.direct_endpoint,
                proxy_endpoint=args.proxy_endpoint,
                session_id=session_id,
            ))

    rows.extend(summary_rows(rows))

    fieldnames = [
        "row_type",
        "design",
        "sequence_index",
        "trial_index",
        "prompt_word_count",
        "prompt_char_count",
        "pair_order",
        "first_endpoint",
        "second_endpoint",
        "direct_position",
        "proxy_position",
        "model",
        "direct_http_status",
        "direct_elapsed_ms",
        "direct_prompt_tokens",
        "direct_output_tokens",
        "direct_total_tokens",
        "direct_reported_total_duration_ms",
        "direct_reported_prompt_eval_ms",
        "direct_reported_eval_ms",
        "proxy_http_status",
        "proxy_elapsed_ms",
        "proxy_prompt_tokens",
        "proxy_output_tokens",
        "proxy_total_tokens",
        "proxy_reported_total_duration_ms",
        "proxy_reported_prompt_eval_ms",
        "proxy_reported_eval_ms",
        "elapsed_delta_ms",
        "elapsed_ratio",
        "response_match",
        "error",
        "summary_metric",
        "summary_value",
    ]

    with output_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)

    print(f"Benchmark complete. Wrote {len(rows)} rows to {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
