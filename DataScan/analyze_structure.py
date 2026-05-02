"""Deeper structural analysis of movie folders."""
import csv
from collections import defaultdict, Counter

CSV_PATH = r'C:\VS\2026_05_01_Movies\DataScan\disk_scan.csv'
VIDEO_EXT = {'.mkv', '.avi', '.mp4', '.webm', '.wmv', '.ts', '.mov', '.mpg', '.mpeg', '.m4v', '.flv', '.iso', '.vob'}

rows = []
with open(CSV_PATH, 'r', encoding='utf-8-sig') as f:
    rdr = csv.DictReader(f)
    for r in rdr:
        rows.append(r)

# Find the depth pattern of video files in each top-level folder
print("=== Depth pattern of video files (where in the tree the actual .mkv/.avi files sit) ===\n")
for top in sorted(set(r['Path'].split('\\')[0] for r in rows)):
    depth_counts = Counter()
    for r in rows:
        if r['Type'] == 'FILE' and r['Ext'].lower() in VIDEO_EXT:
            parts = r['Path'].split('\\')
            if parts[0] == top:
                depth_counts[len(parts)] += 1
    if depth_counts:
        depths = ', '.join(f"depth={d}: {c}" for d, c in sorted(depth_counts.items()))
        print(f"  {top:<35} {depths}")

# Look at DoNotDelete subfolders
print("\n=== DoNotDelete subfolders (depth 2) ===")
for r in rows:
    parts = r['Path'].split('\\')
    if parts[0] == 'DoNotDelete' and len(parts) == 2 and r['Type'] == 'DIR':
        # count videos under this subfolder
        subpath = r['Path'] + '\\'
        videos = sum(1 for x in rows if x['Type'] == 'FILE' and x['Ext'].lower() in VIDEO_EXT and x['Path'].startswith(subpath))
        print(f"  {parts[1]:<35} {videos:>4} videos")

# Check video files in Games and Install (incidental videos to exclude)
print("\n=== Sample video files in non-movie folders (should be excluded) ===")
for top in ('Games', 'Install', 'zzz'):
    print(f"\n  --- {top} ---")
    count = 0
    for r in rows:
        if r['Type'] == 'FILE' and r['Ext'].lower() in VIDEO_EXT:
            parts = r['Path'].split('\\')
            if parts[0] == top and count < 5:
                print(f"    {r['Path']}  ({r['Size_MB']} MB)")
                count += 1

# Check 99 Гледани
print("\n=== 99 Гледани structure (sample) ===")
count = 0
for r in rows:
    parts = r['Path'].split('\\')
    if parts[0] == '99 Гледани' and count < 15:
        prefix = '  ' + '  ' * (len(parts) - 1)
        marker = '[D]' if r['Type'] == 'DIR' else '[F]'
        print(f"{prefix}{marker} {parts[-1]}")
        count += 1

# Check 02_ structure (has depth 3 - so videos at depth 3 means MovieFolder/video.mkv)
print("\n=== 02_ structure sample ===")
count = 0
for r in rows:
    parts = r['Path'].split('\\')
    if parts[0] == '02_' and count < 15:
        prefix = '  ' + '  ' * (len(parts) - 1)
        marker = '[D]' if r['Type'] == 'DIR' else '[F]'
        print(f"{prefix}{marker} {parts[-1]}")
        count += 1

# Look for "Tatko" and "Download"
print("\n=== Tatko and Download folders ===")
for top in ('Tatko', 'Download'):
    items = [r for r in rows if r['Path'].split('\\')[0] == top]
    print(f"  {top}: {len(items)} items total")
    for r in items[:5]:
        print(f"    {r['Path']}")

# Multi-video folders - movie folders that contain MORE THAN ONE video file (could indicate special handling)
print("\n=== Folders with multiple video files (potential issue: which one is THE movie?) ===")
folder_videos = defaultdict(list)
for r in rows:
    if r['Type'] == 'FILE' and r['Ext'].lower() in VIDEO_EXT:
        parts = r['Path'].split('\\')
        if len(parts) >= 2:
            parent = '\\'.join(parts[:-1])
            folder_videos[parent].append((parts[-1], r['Size_MB']))

multi = [(k, v) for k, v in folder_videos.items() if len(v) > 1]
print(f"  Total folders with >1 video: {len(multi)}")
for parent, vids in sorted(multi)[:10]:
    print(f"  {parent}  ({len(vids)} videos)")
    for name, size in vids[:3]:
        print(f"    - {name} ({size} MB)")
