#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
from pathlib import Path
import ipaddress
import re
import sys

paths = [
    Path('docs/proxy-node-deployment-verification.md'),
    Path('internal/api'),
    Path('internal/deploy'),
    Path('scripts'),
    Path('web/src/pages/Deployments.tsx'),
]
files = []
for path in paths:
    if path.is_file():
        files.append(path)
    elif path.is_dir():
        files.extend(file for file in path.rglob('*') if file.is_file())
files = [file for file in files if file.name != 'check-deployment-sensitive-data.sh']

allowed_public_ips = {'1.1.1.1', '8.8.8.8', '223.5.5.5'}
documentation_networks = [
    ipaddress.ip_network('192.0.2.0/24'),
    ipaddress.ip_network('198.51.100.0/24'),
    ipaddress.ip_network('203.0.113.0/24'),
    ipaddress.ip_network('198.18.0.0/15'),
]
forbidden_patterns = [
    ('local absolute path', re.compile(r'/(?:root|home)/[^\s`"\']+')),
    ('temporary forwarding port', re.compile(r'\b(?:57219|64415)\b')),
    ('private key material', re.compile(r'BEGIN (?:OPENSSH|RSA|EC|PRIVATE) KEY')),
    ('credential token', re.compile(r'\b(?:ghp_|github_pat_|sk-)[A-Za-z0-9_-]{10,}')),
]
doc_only_patterns = [
    ('run identifier', re.compile(r'\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}\b')),
    ('environment evidence wording', re.compile(r'Current (?:local|live|browser|real)|real VPS verification evidence|passed locally')),
]

findings = []
for path in files:
    text = path.read_text(errors='ignore')
    for line_number, line in enumerate(text.splitlines(), 1):
        for token in re.findall(r'(?<![0-9])(?:[0-9]{1,3}\.){3}[0-9]{1,3}(?![0-9])', line):
            try:
                ip = ipaddress.ip_address(token)
            except ValueError:
                continue
            allowed = (
                ip.is_loopback
                or ip.is_private
                or ip.is_unspecified
                or token in allowed_public_ips
                or any(ip in network for network in documentation_networks)
            )
            if not allowed:
                findings.append((path, line_number, 'public IP', token))
        for label, pattern in forbidden_patterns:
            match = pattern.search(line)
            if match:
                findings.append((path, line_number, label, match.group(0)))
        if path.name == 'proxy-node-deployment-verification.md':
            for label, pattern in doc_only_patterns:
                match = pattern.search(line)
                if match:
                    findings.append((path, line_number, label, match.group(0)))

if findings:
    for path, line_number, label, value in findings:
        print(f'{path}:{line_number}: forbidden {label}: {value}', file=sys.stderr)
    raise SystemExit(1)

print('Deployment sensitive-data check passed')
PY
