#!/bin/bash
set -e

echo "==================================="
echo "Local Release Testing Script"
echo "==================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if we're in the right directory
if [ ! -f "package.json" ] || [ ! -f ".releaserc.yml" ]; then
    echo -e "${RED}Error: Must run from project root directory${NC}"
    exit 1
fi

echo -e "\n${YELLOW}Step 1: Installing dependencies...${NC}"
if [ ! -d "node_modules" ]; then
    npm install
else
    echo "Dependencies already installed"
fi

echo -e "\n${YELLOW}Step 2: Checking current version...${NC}"
CURRENT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "No tags found")
echo "Current latest tag: $CURRENT_TAG"

echo -e "\n${YELLOW}Step 3: Analyzing recent commits...${NC}"
if [ "$CURRENT_TAG" != "No tags found" ]; then
    echo "Commits since $CURRENT_TAG:"
    git log --oneline "$CURRENT_TAG"..HEAD | head -10
else
    echo "Recent commits:"
    git log --oneline -10
fi

echo -e "\n${YELLOW}Step 4: Testing semantic-release (dry-run)...${NC}"
echo "This will show what version would be released based on commits..."
echo ""

# Create a temporary GitHub token for dry-run (won't actually push)
export GITHUB_TOKEN=${GITHUB_TOKEN:-"dummy-token-for-dry-run"}

# Run semantic-release in dry-run mode
npx semantic-release --dry-run --no-ci --branches "$(git branch --show-current)" 2>&1 | tee /tmp/semantic-release-test.log | grep -E "(semantic-release|new version|No release|Would release)" || true

echo -e "\n${YELLOW}Step 5: Checking what would happen...${NC}"

# Check if a release would be created
if grep -q "There are no relevant changes" /tmp/semantic-release-test.log 2>/dev/null; then
    echo -e "${YELLOW}No release would be created (no feat/fix commits since last release)${NC}"
elif grep -q "This test run was triggered on the branch" /tmp/semantic-release-test.log 2>/dev/null; then
    echo -e "${YELLOW}Note: You're on a feature branch. Semantic-release only creates releases from main/master${NC}"
    echo "To test what would happen on main, you can:"
    echo "  1. Create a test branch from main"
    echo "  2. Cherry-pick your commits"
    echo "  3. Run this script again"
elif grep -q "Published release" /tmp/semantic-release-test.log 2>/dev/null; then
    echo -e "${GREEN}A new release would be created!${NC}"
    grep "Published release" /tmp/semantic-release-test.log
fi

echo -e "\n${YELLOW}Step 6: Testing GoReleaser configuration...${NC}"
if command -v goreleaser &> /dev/null; then
    echo "Testing GoReleaser config (syntax check)..."
    goreleaser check
    echo -e "${GREEN}GoReleaser configuration is valid${NC}"
else
    echo "GoReleaser not installed locally. Install with:"
    echo "  brew install goreleaser"
fi

echo -e "\n${YELLOW}Step 7: Checking required secrets...${NC}"
echo "The following secrets need to be set in GitHub:"
echo ""
echo "For Semantic Release:"
echo "  □ GH_PAT - GitHub Personal Access Token"
echo ""
echo "For Homebrew (GoReleaser):"
echo "  □ MACOS_CERTIFICATE_P12"
echo "  □ MACOS_CERTIFICATE_PASSWORD"
echo "  □ MACOS_NOTARIZATION_ISSUER_ID"
echo "  □ MACOS_NOTARIZATION_KEY_ID"
echo "  □ MACOS_NOTARIZATION_KEY"
echo "  □ BUGSNAG_API_KEY"
echo ""
echo "For PyPI:"
echo "  □ PYPI_API_TOKEN"

echo -e "\n${YELLOW}Step 8: Validating workflow files...${NC}"
for workflow in .github/workflows/*.yml; do
    if npx yaml-lint "$workflow" 2>/dev/null; then
        echo -e "${GREEN}✓${NC} $(basename "$workflow")"
    else
        echo -e "${RED}✗${NC} $(basename "$workflow") - has syntax errors"
    fi
done

echo -e "\n${GREEN}==================================="
echo "Local Testing Complete!"
echo "===================================${NC}"
echo ""
echo "Next steps:"
echo "1. Ensure all commits follow conventional format (feat:, fix:, etc.)"
echo "2. Set up required GitHub secrets"
echo "3. Merge to main branch"
echo "4. Watch GitHub Actions for automatic release"
echo ""
echo "To test a specific version bump:"
echo "  - Minor: git commit -m 'feat: add new feature'"
echo "  - Patch: git commit -m 'fix: resolve bug'"
echo "  - Major: git commit -m 'feat!: breaking change'"
