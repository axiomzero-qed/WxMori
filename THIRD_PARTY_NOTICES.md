# Third-party notices / 第三方声明

**中文：** 每个发布压缩包都会按 Go 模块路径和版本，在 `third-party-licenses/<模块路径>@<版本>/` 中附上 `go list -m -json all` 列出的每个第三方模块根目录中的全部 `LICENSE*`、`COPYING*` 和 `NOTICE*` 文件。项目保留原始文本，不推断或替代模块的许可证。若任一模块没有这些文件，打包会失败并明确指出模块。

**English:** Each release archive includes every root-level `LICENSE*`, `COPYING*`, and `NOTICE*` file for every third-party module listed by `go list -m -json all`, under `third-party-licenses/<module-path>@<version>/`. Original texts are preserved; the project does not infer or replace a module's license. Packaging fails with an explicit error if a module has no such files.

**中文：** WxMori 自身采用 MIT 许可证，见压缩包根目录的 `LICENSE`。请同时遵守所用第三方模块随包提供的声明和许可证。

**English:** WxMori is licensed under MIT; see `LICENSE` at the archive root. Also follow the notices and license terms shipped for the third-party modules you use.
