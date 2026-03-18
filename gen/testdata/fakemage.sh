#!/bin/sh
# fakemage.sh simulates `mage -l` output for use in hermetic tests.
# Only the `-l` flag is recognised; all other invocations exit with an error.
case "$1" in
  -l)
    printf "Targets:\n"
    printf "  build*    build the project (default)\n"
    printf "  format    format codebase using gofmt and goimports\n"
    printf "  lint      lint the code\n"
    printf "  test      run unit tests\n"
    exit 0
    ;;
  *)
    printf "fakemage: unsupported arguments\n" >&2
    exit 1
    ;;
esac
