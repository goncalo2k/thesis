#!/usr/bin/env python3
"""Create a PRISMA-style flow diagram as an SVG asset."""

from __future__ import annotations

import textwrap
import xml.etree.ElementTree as ET
from pathlib import Path


SVG_NS = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG_NS)


WIDTH = 1500
HEIGHT = 2000
STAGE_X = 80
STAGE_WIDTH = 110
STAGE_PADDING = 24
MAIN_X = STAGE_X + STAGE_WIDTH + 140
MAIN_WIDTH = 470
REASON_X = MAIN_X + MAIN_WIDTH + 150
REASON_WIDTH = 380
BOX_HEIGHT = 200
BOX_GAP = 40
MIN_MARGIN = 50
FLOW_PADDING = 28
REASON_PADDING = 24
FLOW_TITLE_WRAP = 24
FLOW_SUBTITLE_WRAP = 30
REASON_TITLE_WRAP = 22
REASON_SUBTITLE_WRAP = 28


FLOW_BOXES = [
    {
        "title": "Records identified through database searching",
        "subtitle": "SCOPUS 1,964 | ACM DL 1,054 | IEEE Xplore 93",
        "value": 3111,
    },
    {
        "title": "Title / abstract / keyword screening passed",
        "subtitle": "Search string applied uniformly across databases",
        "value": 452,
    },
    {
        "title": "Published between 2018 and 2026",
        "subtitle": "Focus on contemporary work aligned with the study scope",
        "value": 364,
    },
    {
        "title": "Written in English",
        "subtitle": "Ensures consistent interpretation of the literature",
        "value": 359,
    },
    {
        "title": "Eligible publication types",
        "subtitle": "Articles, conference papers, short papers, reviews, conference reviews",
        "value": 356,
    },
    {
        "title": ">= 15 citations (papers published before 2025)",
        "subtitle": "Signals a minimum level of peer attention",
        "value": 72,
    },
    {
        "title": "Manual relevance check and deduplication",
        "subtitle": "Full-text assessment of the remaining papers",
        "value": 64,
    },
    {
        "title": "Studies included in the final synthesis",
        "subtitle": "After backward snowballing additions",
        "value": 69,
    },
]


REASON_BOXES = [
    {
        "index": 1,
        "title": "Excluded at title / abstract stage",
        "subtitle": "Did not contain the complete keyword combination",
        "value": 2659,
    },
    {
        "index": 2,
        "title": "Excluded: published before 2018",
        "value": 88,
    },
    {
        "index": 3,
        "title": "Excluded: not written in English",
        "value": 5,
    },
    {
        "index": 4,
        "title": "Excluded: outside accepted publication types",
        "value": 3,
    },
    {
        "index": 5,
        "title": "Excluded: fewer than 15 citations",
        "subtitle": "Applied to papers published before 2025",
        "value": 284,
    },
    {
        "index": 6,
        "title": "Excluded during manual screening",
        "subtitle": "Irrelevant scope or duplicate entries",
        "value": 8,
    },
    {
        "index": 7,
        "title": "Added via backward snowballing",
        "subtitle": "Relevant references discovered while reviewing",
        "value": 5,
        "type": "addition",
    },
]


STAGES = [
    {"label": "Identification", "start": 0, "end": 1},
    {"label": "Screening", "start": 2, "end": 4},
    {"label": "Eligibility", "start": 5, "end": 6},
    {"label": "Included", "start": 7, "end": 7},
]
STAGE_GAP = 30


def rounded_rect(parent, x, y, width, height, *, fill="#fff", stroke="#3a4a5a", stroke_width=2, radius=12):
    return ET.SubElement(
        parent,
        "{%s}rect" % SVG_NS,
        {
            "x": f"{x}",
            "y": f"{y}",
            "width": f"{width}",
            "height": f"{height}",
            "rx": f"{radius}",
            "ry": f"{radius}",
            "fill": fill,
            "stroke": stroke,
            "stroke-width": str(stroke_width),
        },
    )


def add_text_lines(parent, center_x, top_y, lines, *, size=16, weight="normal", color="#1b1b1b", line_height=21):
    if not lines:
        return top_y
    baseline = top_y + size
    for line in lines:
        text = ET.SubElement(
            parent,
            "{%s}text" % SVG_NS,
            {
                "x": f"{center_x}",
                "y": f"{baseline}",
                "text-anchor": "middle",
                "font-family": "Helvetica, Arial, sans-serif",
                "font-size": str(size),
                "font-weight": weight,
                "fill": color,
            },
        )
        text.text = line
        baseline += line_height
    return top_y + size + (len(lines) - 1) * line_height


def wrap(text: str, width: int) -> list[str]:
    if not text:
        return []
    return textwrap.wrap(text, width=width)


def block_height(lines, font_size, line_height):
    if not lines:
        return 0
    return font_size + (len(lines) - 1) * line_height


def add_flow_box(svg, box, y, center_x):
    rounded_rect(svg, MAIN_X, y, MAIN_WIDTH, BOX_HEIGHT)
    title_lines = wrap(box["title"], FLOW_TITLE_WRAP)
    subtitle_lines = wrap(box.get("subtitle", ""), FLOW_SUBTITLE_WRAP)
    value_lines = [f"n = {box['value']:,}"]
    blocks = [
        {
            "lines": title_lines,
            "size": 18,
            "line_height": 24,
            "weight": "600",
            "color": "#1b1b1b",
        },
        {
            "lines": subtitle_lines,
            "size": 14,
            "line_height": 20,
            "weight": "normal",
            "color": "#425466",
        },
        {
            "lines": value_lines,
            "size": 18,
            "line_height": 22,
            "weight": "600",
            "color": "#1b4864",
        },
    ]
    block_gap = 16
    visible_blocks = [b for b in blocks if b["lines"]]
    total_height = sum(block_height(b["lines"], b["size"], b["line_height"]) for b in visible_blocks)
    total_gap = block_gap * (len(visible_blocks) - 1) if visible_blocks else 0
    usable_height = BOX_HEIGHT - FLOW_PADDING * 2
    offset = max(0, (usable_height - (total_height + total_gap)) / 2)
    current_top = y + FLOW_PADDING + offset
    for idx, block in enumerate(visible_blocks):
        current_top = add_text_lines(
            svg,
            center_x,
            current_top,
            block["lines"],
            size=block["size"],
            weight=block["weight"],
            color=block["color"],
            line_height=block["line_height"],
        )
        if idx != len(visible_blocks) - 1:
            current_top += block_gap


def add_reason_box(svg, box, y):
    x = REASON_X
    if box.get("type") == "addition":
        fill = "#e3f6ed"
        stroke = "#208b6b"
    else:
        fill = "#fff9f1"
        stroke = "#c97b30"
    rounded_rect(svg, x, y, REASON_WIDTH, BOX_HEIGHT, fill=fill, stroke=stroke)
    title_lines = wrap(box["title"], REASON_TITLE_WRAP)
    subtitle_lines = wrap(box.get("subtitle", ""), REASON_SUBTITLE_WRAP)
    value_prefix = "+" if box.get("type") == "addition" else "n ="
    value_lines = [f"{value_prefix} {box['value']:,}"]
    blocks = [
        {
            "lines": title_lines,
            "size": 15,
            "line_height": 20,
            "weight": "600",
            "color": "#4a2900" if box.get("type") != "addition" else "#13543f",
        },
        {
            "lines": subtitle_lines,
            "size": 13,
            "line_height": 18,
            "weight": "normal",
            "color": "#59493a" if box.get("type") != "addition" else "#1f5d4b",
        },
        {
            "lines": value_lines,
            "size": 16,
            "line_height": 20,
            "weight": "600",
            "color": "#13543f" if box.get("type") == "addition" else "#7b3f00",
        },
    ]
    block_gap = 14
    visible_blocks = [b for b in blocks if b["lines"]]
    total_height = sum(block_height(b["lines"], b["size"], b["line_height"]) for b in visible_blocks)
    total_gap = block_gap * (len(visible_blocks) - 1) if visible_blocks else 0
    usable_height = BOX_HEIGHT - REASON_PADDING * 2
    offset = max(0, (usable_height - (total_height + total_gap)) / 2)
    current_top = y + REASON_PADDING + offset
    center_x = x + REASON_WIDTH / 2
    for idx, block in enumerate(visible_blocks):
        current_top = add_text_lines(
            svg,
            center_x,
            current_top,
            block["lines"],
            size=block["size"],
            weight=block["weight"],
            color=block["color"],
            line_height=block["line_height"],
        )
        if idx != len(visible_blocks) - 1:
            current_top += block_gap


def add_stage_bar(svg, stage, y_positions, index, total_stages):
    start = y_positions[stage["start"]] - STAGE_PADDING
    end = y_positions[stage["end"]] + BOX_HEIGHT + STAGE_PADDING
    if index > 0:
        start += STAGE_GAP / 2
    if index < total_stages - 1:
        end -= STAGE_GAP / 2
    height = end - start
    rounded_rect(
        svg,
        STAGE_X,
        start,
        STAGE_WIDTH,
        height,
        fill="#d8effb",
        stroke="#69a6c4",
        radius=18,
    )
    center_x = STAGE_X + STAGE_WIDTH / 2
    center_y = start + height / 2
    text = ET.SubElement(
        svg,
        "{%s}text" % SVG_NS,
        {
            "x": f"{center_x}",
            "y": f"{center_y}",
            "text-anchor": "middle",
            "font-family": "Helvetica, Arial, sans-serif",
            "font-size": "18",
            "font-weight": "600",
            "fill": "#2b5c73",
            "transform": f"rotate(-90 {center_x} {center_y})",
        },
    )
    text.text = stage["label"]


def add_marker_defs(svg):
    defs = ET.SubElement(svg, "{%s}defs" % SVG_NS)
    marker = ET.SubElement(
        defs,
        "{%s}marker" % SVG_NS,
        {
            "id": "arrowhead",
            "markerWidth": "10",
            "markerHeight": "10",
            "refX": "4",
            "refY": "3",
            "orient": "auto",
            "markerUnits": "strokeWidth",
        },
    )
    ET.SubElement(marker, "{%s}path" % SVG_NS, {"d": "M0,0 L6,3 L0,6 Z", "fill": "#3a4a5a"})
    return defs


def add_vertical_connectors(svg, y_positions):
    center_x = MAIN_X + MAIN_WIDTH / 2
    for idx in range(len(FLOW_BOXES) - 1):
        y1 = y_positions[idx] + BOX_HEIGHT
        y2 = y_positions[idx + 1]
        ET.SubElement(
            svg,
            "{%s}line" % SVG_NS,
            {
                "x1": f"{center_x}",
                "y1": f"{y1}",
                "x2": f"{center_x}",
                "y2": f"{y2}",
                "stroke": "#3a4a5a",
                "stroke-width": "2.5",
                "marker-end": "url(#arrowhead)",
            },
        )


def add_reason_connectors(svg, y_positions):
    center_x = MAIN_X + MAIN_WIDTH / 2
    for box in REASON_BOXES:
        idx = box["index"]
        y = y_positions[idx] + BOX_HEIGHT / 2
        if box.get("type") == "addition":
            start_x = REASON_X
            elbow_x = MAIN_X + MAIN_WIDTH + 30
            top_y = y_positions[idx] - 18
            path_d = (
                f"M {start_x} {y} H {elbow_x} V {top_y} "
                f"H {center_x} V {y_positions[idx]}"
            )
            ET.SubElement(
                svg,
                "{%s}path" % SVG_NS,
                {
                    "d": path_d,
                    "stroke": "#208b6b",
                    "stroke-width": "2.5",
                    "fill": "none",
                    "marker-end": "url(#arrowhead)",
                },
            )
        else:
            ET.SubElement(
                svg,
                "{%s}line" % SVG_NS,
                {
                    "x1": f"{MAIN_X + MAIN_WIDTH}",
                    "y1": f"{y}",
                    "x2": f"{REASON_X}",
                    "y2": f"{y}",
                    "stroke": "#c97b30",
                    "stroke-width": "2",
                    "marker-end": "url(#arrowhead)",
                },
            )


def build_svg():
    svg = ET.Element("{%s}svg" % SVG_NS, {"width": str(WIDTH), "height": str(HEIGHT), "viewBox": f"0 0 {WIDTH} {HEIGHT}"})
    add_marker_defs(svg)
    total_flow_height = len(FLOW_BOXES) * BOX_HEIGHT + (len(FLOW_BOXES) - 1) * BOX_GAP
    top_margin = max(MIN_MARGIN, (HEIGHT - total_flow_height) / 2)
    y_positions = []
    current_y = top_margin
    for _ in FLOW_BOXES:
        y_positions.append(current_y)
        current_y += BOX_HEIGHT + BOX_GAP
    total_stages = len(STAGES)
    for idx, stage in enumerate(STAGES):
        add_stage_bar(svg, stage, y_positions, idx, total_stages)
    center_x = MAIN_X + MAIN_WIDTH / 2
    for idx, box in enumerate(FLOW_BOXES):
        add_flow_box(svg, box, y_positions[idx], center_x)
    for box in REASON_BOXES:
        idx = box["index"]
        add_reason_box(svg, box, y_positions[idx])
    add_vertical_connectors(svg, y_positions)
    add_reason_connectors(svg, y_positions)
    return svg


def main():
    svg = build_svg()
    output_path = Path("document/assets/prisma_diagram.svg")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    tree = ET.ElementTree(svg)
    tree.write(output_path, encoding="utf-8", xml_declaration=True)
    print(f"Wrote {output_path}")


if __name__ == "__main__":
    main()
