# 放入 2.0 仓库的建议位置

```text
docs/product/p0-link-map.md
docs/architecture/technical-skeleton.md
docs/architecture/p0-data-model-catalog.md
docs/api/p0-api-catalog.yaml
docs/testing/p0-acceptance-principles.md

tasks/iteration-00-contract-freeze.md
...
tasks/iteration-09-p0-e2e-acceptance.md

docs/iterations/iteration-00/
...
docs/iterations/iteration-09/
```

UI Frame 可集中放在：

```text
docs/prototypes/p0-frames/<FRAME_ID>/
```

迭代目录中的 `ui-manifest.json` 只引用这些 Frame，避免生产仓库重复保存图片和 HTML。
