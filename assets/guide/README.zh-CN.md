# pdf-watcher 使用指南

把坚果云（或任意本地）文件夹里**新出现的 PDF / DOCX** 自动整理进 Zotero 10：抽标识符 → Crossref / PubMed 查题录 → 本地 API 建条目并上传附件。

## 开始之前

1. 安装并打开 **Zotero 10+**
2. **设置 → 高级**：勾选「允许本计算机上的其他应用程序与 Zotero 通信」
3. 运行 `pdf-watcher.exe`，在托盘（任务栏右下角 ^）找到图标，**右键**打开菜单
4. 第一次写入时，在 Zotero 弹窗选 **Always Allow**（不要只点 Allow）

## 日常用法

1. 托盘 → **添加监控文件夹**（只监控该文件夹**本层**，不含子目录）
2. 把 PDF/DOCX 放进该文件夹（或同步进来）
3. 稍等文件写完：程序会建条目、挂附件
4. 文集位置：Zotero 左侧 **pdf-watcher → 文件夹名**（同名目录冲突时会变成 `父目录/文件夹名`）

## 题录从哪来

- **真源**：Crossref / PubMed（可双核查：DOI 与 PubMed 结果一致才采用）
- **大模型（可选）**：只在找不到 ID 时帮忙找 DOI/PMID，**不**直接当题录
- 配置 `ai`（OpenAI 兼容接口）：填 `ai.apiKey`，且 `ai.enable: true` 才启用找 ID；`apiUrl` 换成任意兼容 `/v1/chat/completions` 的地址即可（DeepSeek / OpenAI / 通义等）

## 配置与数据位置

全部在用户目录，不写 exe 旁边：

| 内容 | 路径 |
|------|------|
| 配置 | `%APPDATA%\pdf-watcher\config.yaml` |
| 日志 | `%APPDATA%\pdf-watcher\logs\` |
| 入库索引 | `%APPDATA%\pdf-watcher\index.db` |

- 配置会**热加载**，改完保存即可，一般不用重启
- **不要手改** `collectionId` / `collectionName`（程序维护）
- **不要轻易删** `index.db`：删了不会丢 Zotero 文献，但隔几天再丢附件时可能挂不到旧条目

## 同文献后补附件

只要索引里已有该 DOI/PMID（或足够长的规范化标题），几天后再丢另一个文件会**挂到同一父条目**下，而不是新建一条。程序只记得**自己成功入库过**的条目。

## 常见问题

**没装 / 没开 Zotero**  
程序仍会正常启动托盘，不会崩溃。菜单会显示「Zotero：不可用」，悬停提示也会说明；装好并打开 Zotero 10（且开启本机通信）后会自动变为已连接，积压文件再处理。

**托盘没图标**  
看任务栏 ^ 隐藏区；或结束旧进程后重新运行 exe。

**日志里 403 / local API disabled**  
Zotero 高级设置里打开本机通信。

**need Always Allow**  
写入授权选 Always Allow；选完后配置里会出现 `zotero.apiKey`。

**附件打不开 / 找不到文件**  
多半是上传未完成留下的空壳：删掉该条目，把文件再放进监控目录触发一次。

**导出 Zotero 文集**  
导出文集列表（path/name/key）并用默认程序打开。日常仍靠自动 `pdf-watcher/文件夹`，一般不必用。

## 编译

```text
go build -ldflags="-H=windowsgui -s -w" -o pdf-watcher.exe .
```
