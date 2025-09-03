#!/usr/bin/env python3
"""
Local Documentation Check Script for MetalLB

This script can be run locally to test the documentation checking logic.
Usage: python3 scripts/check_docs_local.py [base_branch]

Example: python3 scripts/check_docs_local.py origin/main
"""

import os
import sys
import subprocess
import re
from typing import List, Dict, Set, Tuple

class DocumentationChecker:
    def __init__(self, base_ref: str = "origin/main"):
        self.base_ref = base_ref
        self.warnings = []
        self.info_messages = []

    def run_git_command(self, cmd: List[str]) -> str:
        """Run a git command and return its output."""
        try:
            result = subprocess.run(
                cmd, capture_output=True, text=True, check=True
            )
            return result.stdout.strip()
        except subprocess.CalledProcessError as e:
            print(f"Git command failed: {' '.join(cmd)}")
            print(f"Error: {e.stderr}")
            return ""

    def get_changed_files(self) -> List[str]:
        """Get list of files changed compared to base branch."""
        cmd = ["git", "diff", "--name-only", f"{self.base_ref}..HEAD"]
        output = self.run_git_command(cmd)
        return [f for f in output.split('\n') if f.strip()]

    def get_added_files(self) -> List[str]:
        """Get list of files added compared to base branch."""
        cmd = ["git", "diff", "--name-status", f"{self.base_ref}..HEAD"]
        output = self.run_git_command(cmd)
        added_files = []
        for line in output.split('\n'):
            if line.startswith('A\t'):
                added_files.append(line[2:])
        return added_files

    def get_file_diff(self, filepath: str) -> str:
        """Get the diff for a specific file."""
        cmd = ["git", "diff", f"{self.base_ref}..HEAD", "--", filepath]
        return self.run_git_command(cmd)

    def detect_api_changes(self, changed_files: List[str]) -> Dict[str, List[str]]:
        """Detect API changes that likely need documentation."""
        api_changes = {
            'new_crd_fields': [],
            'new_crd_files': [],
            'version_changes': [],
            'spec_changes': []
        }

        for file in changed_files:
            if file.startswith('api/') and file.endswith('_types.go'):
                diff = self.get_file_diff(file)

                # Check for new struct fields (lines starting with +)
                for line in diff.split('\n'):
                    if line.startswith('+') and not line.startswith('+++'):
                        # Look for new struct fields
                        if re.search(r'\+\s+\w+\s+\w+.*`json:', line):
                            api_changes['new_crd_fields'].append(f"{file}: {line.strip()[1:].strip()}")
                        # Look for new kubebuilder annotations
                        if '+kubebuilder:' in line:
                            api_changes['spec_changes'].append(f"{file}: {line.strip()[1:].strip()}")

                # Check if it's a new API version file
                if file in self.get_added_files():
                    api_changes['new_crd_files'].append(file)

                # Check for version changes
                if 'v1beta' in file and ('v1beta2' in diff or 'v1beta3' in diff):
                    api_changes['version_changes'].append(file)

        return api_changes

    def detect_config_changes(self, changed_files: List[str]) -> Dict[str, List[str]]:
        """Detect configuration changes that likely need documentation."""
        config_changes = {
            'new_config_samples': [],
            'helm_chart_changes': [],
            'crd_changes': []
        }

        for file in changed_files:
            # New configuration samples
            if file.startswith('configsamples/') and file.endswith('.yaml'):
                if file in self.get_added_files():
                    config_changes['new_config_samples'].append(file)

            # Helm chart changes
            if file.startswith('charts/metallb/'):
                if 'values.yaml' in file or 'templates/' in file:
                    diff = self.get_file_diff(file)
                    # Look for new configuration options
                    for line in diff.split('\n'):
                        if line.startswith('+') and not line.startswith('+++'):
                            if ':' in line and not line.strip().startswith('# '):
                                config_changes['helm_chart_changes'].append(f"{file}: {line.strip()[1:].strip()}")

            # CRD base changes (generated files)
            if file.startswith('config/crd/bases/') and file.endswith('.yaml'):
                config_changes['crd_changes'].append(file)

        return config_changes

    def detect_documentation_updates(self, changed_files: List[str]) -> Dict[str, List[str]]:
        """Detect what documentation has been updated."""
        doc_updates = {
            'api_docs': [],
            'configuration_docs': [],
            'usage_docs': [],
            'concept_docs': [],
            'examples': []
        }

        for file in changed_files:
            if file.startswith('website/content/'):
                if 'apis/' in file:
                    doc_updates['api_docs'].append(file)
                elif 'configuration/' in file:
                    doc_updates['configuration_docs'].append(file)
                elif 'usage/' in file:
                    doc_updates['usage_docs'].append(file)
                elif 'concepts/' in file:
                    doc_updates['concept_docs'].append(file)

            if file.startswith('configsamples/'):
                doc_updates['examples'].append(file)

        return doc_updates

    def analyze_feature_additions(self) -> Tuple[bool, List[str]]:
        """Analyze if current changes add features that need documentation."""
        changed_files = self.get_changed_files()

        if not changed_files:
            return False, []

        self.info_messages.append(f"📁 Analyzing {len(changed_files)} changed files...")

        api_changes = self.detect_api_changes(changed_files)
        config_changes = self.detect_config_changes(changed_files)
        doc_updates = self.detect_documentation_updates(changed_files)

        has_feature_changes = False
        feature_details = []

        # Check API changes
        if api_changes['new_crd_fields']:
            has_feature_changes = True
            feature_details.append("New CRD fields detected:")
            for change in api_changes['new_crd_fields']:
                feature_details.append(f"  - {change}")

        if api_changes['new_crd_files']:
            has_feature_changes = True
            feature_details.append("New CRD files detected:")
            for change in api_changes['new_crd_files']:
                feature_details.append(f"  - {change}")

        if api_changes['spec_changes']:
            has_feature_changes = True
            feature_details.append("New API specifications detected:")
            for change in api_changes['spec_changes']:
                feature_details.append(f"  - {change}")

        # Check config samples changes
        if config_changes['new_config_samples']:
            has_feature_changes = True
            feature_details.append("New configuration samples detected:")
            for change in config_changes['new_config_samples']:
                feature_details.append(f"  - {change}")

        # Check Helm chart changes
        if config_changes['helm_chart_changes']:
            has_feature_changes = True
            feature_details.append("New Helm chart changes detected:")
            for change in config_changes['helm_chart_changes'][:5]:  # Limit to first 5
                feature_details.append(f"  - {change}")
            if len(config_changes['helm_chart_changes']) > 5:
                feature_details.append(f"  - ... and {len(config_changes['helm_chart_changes']) - 5} more")

        # Report documentation updates
        doc_sections_updated = []
        if doc_updates['api_docs']:
            doc_sections_updated.append("API documentation")
        if doc_updates['configuration_docs']:
            doc_sections_updated.append("Configuration guides")
        if doc_updates['usage_docs']:
            doc_sections_updated.append("Usage documentation")
        if doc_updates['concept_docs']:
            doc_sections_updated.append("Concept documentation")
        if doc_updates['examples']:
            doc_sections_updated.append("Examples")

        if doc_sections_updated:
            self.info_messages.append(f"Documentation updated: {', '.join(doc_sections_updated)}")

        return has_feature_changes, feature_details

    def generate_recommendations(self, feature_details: List[str]) -> List[str]:
        """Generate documentation recommendations based on detected changes."""
        recommendations = []

        # API
        if any("CRD" in detail or "API" in detail for detail in feature_details):
            recommendations.extend([
                "  - Consider updating API documentation in `website/content/apis/`",
                "  - Consider updating configuration guides in `website/content/configuration/`",
                "  - Consider adding usage examples in `website/content/usage/` or `configsamples/`"
            ])

        # Helm
        if any("Helm" in detail for detail in feature_details):
            recommendations.extend([
                "  - Consider documenting new Helm options in configuration guides",
                "  - Consider updating installation documentation if needed"
            ])

        # Samples
        if any("samples" in detail for detail in feature_details):
            recommendations.extend([
                "  - Consider documenting the new configuration patterns",
                "  - Consider adding references to the new examples in usage guides"
            ])

        return recommendations

    def check(self) -> int:
        """Main check function. Returns 0 for success, 1 for warnings."""
        print(f"🔍 Checking changes against base: {self.base_ref}")
        print()

        has_feature_changes, feature_details = self.analyze_feature_additions()

        if not has_feature_changes:
            print("✅ No user-facing feature changes detected. Documentation check passed.")
            return 0

        print("🔍 Feature changes detected:")
        print()
        for detail in feature_details:
            print(detail)
        print()

        for msg in self.info_messages:
            print(msg)
        print()

        recommendations = self.generate_recommendations(feature_details)

        if recommendations:
            print("💡 Documentation Recommendations:")
            print()
            for rec in recommendations:
                print(rec)
            print()

        return 0  # Non-blocking

def main():
    if len(sys.argv) > 1:
        base_ref = sys.argv[1]
    else:
        base_ref = "origin/main"

    # Check if we're in a git repository
    try:
        subprocess.run(["git", "rev-parse", "--git-dir"],
                      capture_output=True, check=True)
    except subprocess.CalledProcessError:
        print("❌ Error: Not in a git repository")
        return 1

    checker = DocumentationChecker(base_ref)
    return checker.check()

if __name__ == "__main__":
    sys.exit(main())
