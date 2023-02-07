# fnruntime

## Description
fnruntime controller

## Usage

### Fetch the package
`kpt pkg get REPO_URI[.git]/PKG_PATH[@VERSION] fnruntime`
Details: https://kpt.dev/reference/cli/pkg/get/

### View package content
`kpt pkg tree fnruntime`
Details: https://kpt.dev/reference/cli/pkg/tree/

### Apply the package
```
kpt live init fnruntime
kpt live apply fnruntime --reconcile-timeout=2m --output=table
```
Details: https://kpt.dev/reference/cli/live/
