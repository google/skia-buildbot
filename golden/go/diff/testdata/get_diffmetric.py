#!/usr/bin/env python

# Usage: get_diffmetric.py img1 img2
#
# Helper script that calculates the DiffMetric between two given images.
# For debugging purposes only.
#
# Note: This requires PIL or Pillow to be installed.


from __future__ import print_function
import sys
from PIL import Image

## Main program.
img1_path = sys.argv[1]
img2_path = sys.argv[2]

img1 = Image.open(img1_path)
img2 = Image.open(img2_path)
has_alpha = (img1.mode == img2.mode)

xMin = min(img1.size[0], img2.size[0])
yMin = min(img1.size[1], img2.size[1])
xMax = max(img1.size[0], img2.size[0])
yMax = max(img1.size[1], img2.size[1])

px_img1 = img1.load()
px_img2 = img2.load()

num_diff_pixel = xMax * yMax
max_rgba_diff = [0, 0, 0, 0]


for x in xrange(xMin):
    for y in xrange(yMin):
        p1 = (px_img1[x, y] + (0xFF,))[:4]
        p2 = (px_img2[x, y] + (0xFF,))[:4]

        if (p1 != p2) or not has_alpha:
            for channel in range(len(p1)):
                pdiff = abs(p1[channel]-p2[channel])
                max_rgba_diff[channel] = max(max_rgba_diff[channel], pdiff)
        else:
            num_diff_pixel -= 1

image_sizes_differ = (
    (img1.size[0] != img2.size[0]) or
    (img1.size[1] != img2.size[1]))
total_px = xMax * yMax
pixel_diff_percent = (num_diff_pixel / float(total_px)) * 100
if pixel_diff_percent > 100.0:
    print("Error: Percent difference > 100%")

print("Image Sizes Differ:", image_sizes_differ)
print("TotalNumPixels:    ", total_px)
print("NumDiffPixels:     ", num_diff_pixel)
print("PixelDiffPercent:  ", pixel_diff_percent)
print("MaxRGBADiffs:      ", max_rgba_diff)
