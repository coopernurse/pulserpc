#!/bin/bash
# tag_release.sh - Create a release tag for PulseRPC
# Usage: ./scripts/tag_release.sh v0.2.0

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if version argument is provided
if [ $# -ne 1 ]; then
    echo -e "${RED}Error: Version argument required${NC}"
    echo "Usage: $0 vX.Y.Z"
    exit 1
fi

VERSION=$1

# Validate version format (must be v followed by semver)
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo -e "${RED}Error: Invalid version format '$VERSION'${NC}"
    echo "Version must be in semver format with 'v' prefix: vX.Y.Z"
    exit 1
fi

echo -e "${GREEN}=== PulseRPC Release Script ===${NC}"
echo -e "Version: ${YELLOW}$VERSION${NC}"
echo ""

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo -e "${RED}Error: Not on main branch (current: $CURRENT_BRANCH)${NC}"
    echo "Releases must be created from the main branch"
    exit 1
fi
echo -e "${GREEN}✓${NC} On main branch"

# Check if working directory is clean (no staged or unstaged changes)
if [ -n "$(git diff --name-only)" ] || [ -n "$(git diff --cached --name-only)" ]; then
    echo -e "${RED}Error: Working directory not clean${NC}"
    echo "Please commit or stash changes before creating a release"
    git status --short
    exit 1
fi
echo -e "${GREEN}✓${NC} Working directory is clean"

# Run quality checks
echo ""
echo -e "${YELLOW}Running quality checks...${NC}"
make quality
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Quality checks failed${NC}"
    exit 1
fi
echo -e "${GREEN}✓${NC} Quality checks passed"

# Run integration tests
echo ""
echo -e "${YELLOW}Running integration tests...${NC}"
make test-generators
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Integration tests failed${NC}"
    exit 1
fi
echo -e "${GREEN}✓${NC} Integration tests passed"

# Update version in cmd/pulse/version.go
echo ""
echo -e "${YELLOW}Updating version in cmd/pulse/version.go...${NC}"
# Detect if we're on macOS (BSD sed) or Linux (GNU sed)
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s/const Version = \".*\"/const Version = \"$VERSION\"/" cmd/pulse/version.go
else
    sed -i "s/const Version = \".*\"/const Version = \"$VERSION\"/" cmd/pulse/version.go
fi
echo -e "${GREEN}✓${NC} Version updated to $VERSION"

# Commit the version change
echo ""
echo -e "${YELLOW}Creating release commit...${NC}"
git add cmd/pulse/version.go
git commit -m "Release version $VERSION"
echo -e "${GREEN}✓${NC} Commit created"

# Create annotated tag
echo ""
echo -e "${YELLOW}Creating git tag...${NC}"
git tag -a "$VERSION" -m "Release version $VERSION"
echo -e "${GREEN}✓${NC} Tag $VERSION created"

# Push to GitHub
echo ""
echo -e "${YELLOW}Pushing to GitHub...${NC}"
git push
git push origin "$VERSION"
echo -e "${GREEN}✓${NC} Pushed to GitHub"

echo ""
echo -e "${GREEN}=== Release $VERSION created successfully! ===${NC}"
