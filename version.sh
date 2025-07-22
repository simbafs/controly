#!/bin/bash

version=$1

if [ -z "$version" ]; then
  echo "Usage: $0 <version>"
  exit 1
fi

# Check if the version is already set in the file
if git tag | grep -q "$version"; then
  echo "Version $version already exists."
  exit 1
fi

cd sdk && npm version "$version"

git add .
git commit -m "Release version $version"
git tag "$version"
