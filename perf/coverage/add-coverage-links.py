#!/usr/bin/env python3
import os
from bs4 import BeautifulSoup

def add_link(file_path, link_text, link_url):
    if not os.path.exists(file_path):
        print(f"File not found: {file_path}")
        return
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    # Use lxml parser for robustness
    soup = BeautifulSoup(content, 'lxml')

    # Remove existing back link if it exists by ID
    existing_link = soup.find('div', id='perf-coverage-back-link')
    if existing_link:
        existing_link.decompose()

    # Also check for old pattern without ID to be thorough
    for div in soup.find_all('div'):
        if div.get('style') and 'text-align: center' in div.get('style'):
            anchor = div.find('a')
            if anchor and anchor.string == link_text:
                div.decompose()

    # Create the new link element
    link_div = soup.new_tag('div', id='perf-coverage-back-link')
    link_div['style'] = 'text-align: center; margin: 2em;'

    link_a = soup.new_tag('a', href=link_url)
    link_a['class'] = 'ui blue button'
    link_a['style'] = 'background-color: #2185d0; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; font-family: sans-serif;'
    link_a.string = link_text

    link_div.append(link_a)

    # Insert at the beginning of the body
    if soup.body:
        soup.body.insert(0, link_div)

        # Write back the modified HTML
        with open(file_path, 'w', encoding='utf-8') as f:
            # We use str(soup) instead of prettify() to minimize unnecessary whitespace changes
            f.write(str(soup))
        print(f"Updated {file_path}")
    else:
        print(f"No <body> tag found in {file_path}")

def main():
    # Base directory for coverage reports
    base_dir = 'perf/coverage-reports'

    # All 3 pages (sub-reports)
    pages = [
        ('type-coverage/index.html', 'Back to Perf Coverage Dashboard', '../'),
        ('test-coverage/index.html', 'Back to Perf Coverage Dashboard', '../'),
        ('mutation-testing/index.html', 'Back to Perf Coverage Dashboard', '../'),
    ]

    for rel_path, text, url in pages:
        full_path = os.path.join(base_dir, rel_path)
        add_link(full_path, text, url)

if __name__ == '__main__':
    main()