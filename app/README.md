# README

## About

This is the official Wails Svelte template.

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

### Local macOS builds

In this checkout, `wails build`'s final ad-hoc codesign step can fail
with:

```
codesign failed: exit status 1 – .../open-rig-programmer: replacing existing signature
.../open-rig-programmer: resource fork, Finder information, or similar detritus not allowed
```

This is `com.apple.provenance`/resource-fork extended attributes stamped
on freshly written files in this (iCloud-synced) path — CI is
unaffected. The build itself has already completed by this point; only
the self-sign step fails. Workaround (verified 12/07/2026):

```sh
xattr -cr "build/bin/Open Rig Programmer.app"
codesign --force --deep -s - "build/bin/Open Rig Programmer.app"
codesign -dv "build/bin/Open Rig Programmer.app"   # confirm: Signature=adhoc
```

Note: `wails build` runs `npm install` (`wails.json`), which can leave
`app/frontend/package-lock.json`/`package.json.md5` (and sometimes
`wailsjs/runtime`'s file mode) showing as modified with no real content
change — `git checkout --` those if you didn't intend a dependency
change; don't commit build collateral.
