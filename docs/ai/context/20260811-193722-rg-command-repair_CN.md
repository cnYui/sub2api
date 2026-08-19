# ripgrep 命令修复记录

## 问题

PowerShell 的 `rg` 优先解析到 `C:\Users\yui\AppData\Local\Microsoft\WinGet\Links\rg.exe`。该路径原本是指向 WinGet ripgrep 安装目录的符号链接，执行时被系统判定为不受信任装入点，导致项目搜索失败；实际目标文件可以直接运行。

## 修复

- 保留原符号链接为 `C:\Users\yui\AppData\Local\Microsoft\WinGet\Links\rg.exe.broken-20260811`，便于回溯。
- 将 `rg.exe` 重建为指向同一目标文件的 NTFS 硬链接，避免通过符号链接执行时触发系统限制。
- 未修改项目源码、依赖、仓库配置或 WinGet 安装目录。

## 验证

- `rg --version`：`ripgrep 15.1.0`，PCRE2 可用。
- 在项目根目录执行 `rg -n --glob 'package.json' '"name"' .` 成功返回 `frontend/package.json`。
- 修复后 `Get-Item` 显示 `LinkType=HardLink`，目标文件 SHA-256 为 `DECDD4992F3F1B9A5EF9898F1B40AB16886D579D6516B4EFD3D5EAA19364E408`。

