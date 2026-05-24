# Releasing

## Steps

1. **Update CHANGELOG.md** — add a section for the new version and date.

2. **Commit all changes**
   ```bash
   git add .
   git commit -m "chore: release vX.Y.Z"
   ```

3. **Build release archives**
   ```bash
   make release   # produces dist/clab_<os>_<arch>.tar.gz
   ```

4. **Tag**
   ```bash
   git tag vX.Y.Z
   ```

5. **Push**
   ```bash
   git push origin main
   git push origin vX.Y.Z
   ```

6. **Create GitHub release**
   ```bash
   NOTES="$(awk '/## \[vX.Y.Z\]/{found=1; next} found && /^## \[/{exit} found{print}' CHANGELOG.md)"
   gh release create vX.Y.Z dist/*.tar.gz \
     --title "vX.Y.Z" \
     --target main \
     --notes "$NOTES"
   ```

   > `--target main` is required when the tag has not yet been pushed to the remote.

## Versioning

Follows [Semantic Versioning](https://semver.org/):

- `MAJOR` — breaking changes (removed rules, changed binary flags, changed output format)
- `MINOR` — new rules, new flags, new binaries
- `PATCH` — bug fixes, doc updates
