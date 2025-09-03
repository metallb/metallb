# MetalLB Documentation Check

This directory contains the documentation checking system for MetalLB, which helps ensure that new features are properly documented.

## How it Works

The documentation check system analyzes PR changes to detect:

### 🔍 **What it Detects as "Features Needing Documentation":**

1. **API Changes:**
   - New fields in CRD specifications (`api/v1beta*/*_types.go`)
   - New kubebuilder annotations
   - New API version files
   - Changes to CRD base files (`config/crd/bases/*.yaml`)

2. **Configuration Changes:**
   - New configuration samples (`configsamples/*.yaml`)
   - New Helm chart options (`charts/metallb/values.yaml`)
   - New Helm templates (`charts/metallb/templates/*.yaml`)

3. **Feature Additions:**
   - New struct fields with JSON tags
   - New configuration parameters
   - New examples or use cases

### 📚 **What it Considers "Documentation Updates":**

1. **API Documentation:** Files in `website/content/apis/`
2. **Configuration Guides:** Files in `website/content/configuration/`
3. **Usage Documentation:** Files in `website/content/usage/`
4. **Concept Documentation:** Files in `website/content/concepts/`
5. **Examples:** Files in `configsamples/`

## Usage

### For Contributors

The check runs automatically on all PRs that modify relevant files. If it detects feature changes without corresponding documentation updates, it will provide recommendations (but won't block the PR).

### For Testing Locally

Run the local version to test your changes before submitting a PR:

```bash
# Test against main branch
python3 scripts/check_docs_local.py

# Test against a specific branch
python3 scripts/check_docs_local.py origin/v0.14

# Test with current uncommitted changes
git add . && python3 scripts/check_docs_local.py HEAD~1
```

### Example Output

```
🔍 Checking changes against base: origin/main

📁 Analyzing 3 changed files...

🔍 **Feature changes detected:**

🔧 **New CRD fields detected:**
  - api/v1beta2/bgppeer_types.go: DynamicASN DynamicASNMode `json:"dynamicASN,omitempty"`

📋 **New configuration samples detected:**
  - configsamples/bgppeer_dynamic_asn.yaml

📚 **Documentation updated:** Examples

💡 **Documentation Recommendations:**

📖 Consider updating API documentation in `website/content/apis/`
📝 Consider updating configuration guides in `website/content/configuration/`
💡 Consider adding usage examples in `website/content/usage/` or `configsamples/`

⚠️  **This is a non-blocking check.** Please consider updating documentation for new features.
📝 If documentation updates are planned for a follow-up PR, please mention it in the PR description.
```

## Integration with CI

The check is integrated into the GitHub Actions workflow in `.github/workflows/doc-check.yaml`. The workflow:

1. **Runs automatically** on PRs that change relevant files
2. **Invokes** the same `scripts/check_docs_local.py` script (no code duplication)
3. **Provides detailed analysis** in the workflow logs
4. **Comments on the PR** with results
5. **Is non-blocking** - it won't prevent PR merging

## File Patterns Monitored

The workflow triggers when these paths are modified:

- `api/**` - API definitions
- `configsamples/**` - Configuration examples  
- `charts/metallb/**` - Helm charts
- `config/crd/**` - Generated CRD files
- `website/content/**` - Documentation files

## Customization

To modify what the checker considers as "feature changes" or "documentation updates", edit the detection logic in:

- `detect_api_changes()` - API change detection
- `detect_config_changes()` - Configuration change detection  
- `detect_documentation_updates()` - Documentation update detection
- `generate_recommendations()` - Recommendation generation

## Implementation Details

The checker uses Git diff analysis to:

1. **Compare** current branch against base branch
2. **Parse** diffs to identify specific change patterns
3. **Categorize** changes as feature additions or documentation updates
4. **Generate** contextual recommendations based on change types
5. **Report** findings in a user-friendly format

This approach ensures that the check is:
- **Accurate** - Only flags actual feature additions
- **Helpful** - Provides specific recommendations
- **Non-intrusive** - Doesn't block development workflow
- **Maintainable** - Easy to update detection patterns

## Future Improvements

Potential enhancements could include:

- Integration with PR templates for mandatory checklists
- More sophisticated change detection (semantic analysis)
- Automatic documentation skeleton generation
- Integration with documentation build process
- Metrics on documentation coverage for features
