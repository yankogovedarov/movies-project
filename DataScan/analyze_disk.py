"""Analyze the disk scan CSV to understand structure."""
import csv
from collections import defaultdict, Counter

CSV_PATH = r'C:\VS\2026_05_01_Movies\DataScan\disk_scan.csv'
VIDEO_EXT = {'.mkv', '.avi', '.mp4', '.webm', '.wmv', '.ts', '.mov', '.mpg', '.mpeg', '.m4v', '.flv', '.iso', '.vob'}

rows = []
with open(CSV_PATH, 'r', encoding='utf-8-sig') as f:
    rdr = csv.DictReader(f)
    for r in rdr:
        rows.append(r)

print(f"Total rows: {len(rows):,}\n")

# Top-level folders: first path segment
top_level = defaultdict(lambda: {'items': 0, 'video_files': 0, 'video_size_gb': 0.0, 'extensions': Counter(), 'max_depth': 0})
for r in rows:
    parts = r['Path'].split('\\')
    top = parts[0]
    depth = len(parts)
    tl = top_level[top]
    tl['items'] += 1
    tl['max_depth'] = max(tl['max_depth'], depth)
    if r['Type'] == 'FILE':
        ext = r['Ext'].lower()
        tl['extensions'][ext] += 1
        if ext in VIDEO_EXT:
            tl['video_files'] += 1
            tl['video_size_gb'] += float(r['Size_MB'].replace(',', '.')) / 1024

print(f"{'Folder':<35} {'Items':>7} {'Videos':>7} {'GB':>8} {'Depth':>6}")
print('-' * 72)
for name, data in sorted(top_level.items()):
    print(f"{name:<35} {data['items']:>7,} {data['video_files']:>7,} {data['video_size_gb']:>8.1f} {data['max_depth']:>6}")

# Total stats
total_videos = sum(d['video_files'] for d in top_level.values())
total_gb = sum(d['video_size_gb'] for d in top_level.values())
print(f"\nTOTAL videos: {total_videos:,}, ~{total_gb:.0f} GB")

# Extension breakdown across the whole disk
print("\n=== Video file extensions globally ===")
all_ext = Counter()
for d in top_level.values():
    for k, v in d['extensions'].items():
        if k in VIDEO_EXT:
            all_ext[k] += v
for ext, count in all_ext.most_common():
    print(f"  {ext}: {count:,}")
