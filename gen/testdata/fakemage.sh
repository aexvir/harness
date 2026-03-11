#!/bin/sh
# Fake mage -l output for testing
case "$1" in
  -l)
    printf "Targets:\n"
    printf "  format    Format code\n"
    printf "  lint      Lint the code\n"
    printf "  test      Run tests\n"
    exit 0
    ;;
  *)
    exit 1
    ;;
esac
