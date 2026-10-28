# miwu-go

用于管理米家账号、家庭、设备和场景的 Go 命令行工具。支持终端选择向导和可脚本化子命令。

## 构建与首次使用

需要 Go 1.26 或更新版本。本轮优先支持 macOS；Windows/Linux 的系统钥匙串集成尚未进行真实验收。

```sh
make build
./bin/miwu --help
./bin/miwu login --qr
./bin/miwu account list
./bin/miwu account use --account YOUR_ACCOUNT_ID
./bin/miwu
```

最后一个命令在交互终端中打开操作菜单；非终端环境显示帮助。菜单支持账号、家庭、设备、属性及动作选择。EOF 或 Ctrl+C 结束交互。

密码登录可以在终端隐藏输入，也可以从标准输入读取；不支持通过命令参数传入明文密码：

```sh
./bin/miwu login --username YOUR_USERNAME
./bin/miwu login --username YOUR_USERNAME --password-stdin < password.txt
```

密码登录遇到验证码或二次验证时，终端会询问是否切换扫码；脚本返回 `verification_required` 错误。扫码轮询超时后，终端可以选择生成新二维码。二维码写入 stderr，不混入 JSON 结果。

## 常用命令

```sh
./bin/miwu account show
./bin/miwu account check
./bin/miwu account refresh
./bin/miwu account delete --account YOUR_ACCOUNT_ID --yes

./bin/miwu home list
./bin/miwu device list --home HOME_ID
./bin/miwu device show --home HOME_ID --did DEVICE_ID

./bin/miwu device get --did DEVICE_ID --property 2.1 --property 2.2
./bin/miwu device set --did DEVICE_ID --set '2.1=false'
./bin/miwu device set --did DEVICE_ID --set '2.2=42' --set '2.3="hello"'
./bin/miwu device action --did DEVICE_ID --siid 2 --aiid 1 --in '[false,42]'

./bin/miwu scene list --home HOME_ID
./bin/miwu scene run --home HOME_ID --scene SCENE_ID

./bin/miwu spec show --urn 'urn:miot-spec-v2:device:light:0000A001:test-lamp:1'
./bin/miwu spec show --urn YOUR_URN --language ''
./bin/miwu spec languages --urn YOUR_URN
./bin/miwu spec icon --model DEVICE_MODEL
./bin/miwu spec parse-urn --urn YOUR_URN
```

属性和动作 IID 取自具体设备的规格，示例中的 ID 不能直接用于任意设备。终端中省略选择参数时会进入向导；属性写入向导展示类型、枚举和范围，检查输入是否合法。没有可用规格时，可输入数字 IID 和 JSON 值。显式 IID 命令直接使用服务端校验，适合规格暂不可用的设备。

只要未指定 `--account`，工具会读取有效默认账号；没有默认账号时，终端提供选择，脚本要求显式参数。家庭列表同时包含自有家庭和共享家庭，共享家庭请求使用家庭所有者的 UID。

## 脚本输出与错误

```sh
./bin/miwu device list \
  --account YOUR_ACCOUNT_ID --home HOME_ID \
  --output json --non-interactive
```

`--output json` 自动禁止提示输入，输出固定包装：

```json
{"data":{"devices":[],"has_more":false,"cursor":""},"error":null}
```

失败示例：

```json
{"data":null,"error":{"kind":"business","message":"...","business_code":-6}}
```

批量属性操作部分失败时，`data` 保留所有逐项结果，`error.kind` 为 `partial_failure`。大整数值保留原始精度；账号、家庭和家庭所有者 ID 在输出中使用字符串。

| 退出码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 1 | 运行失败，查看错误类别和原因 |
| 2 | 参数错误或缺少非交互参数 |
| 3 | 批量操作部分失败 |
| 130 | 用户取消、上下文取消或输入结束 |

stdout 只用于结果，stderr 用于菜单、二维码、诊断和文本错误。表格模式用于列表，复杂对象以缩进 JSON 展示。`--verbose` 只记录方法、主机、HTTP 状态和是否失败，不记录认证 URL 查询、Cookie、请求或响应正文。设备 Token 和登录凭据不会出现在输出中。

写属性、动作和场景不会自动重试。超时或无法确认响应时可能返回 `outcome_unknown`：远程操作可能已经发生，工具没有重复发送。场景成功信息只代表请求已被接受。

## 配置与账号文件

默认根目录为 `os.UserConfigDir()/miwu`，macOS 通常是 `~/Library/Application Support/miwu`：

- `config.yaml`：默认账号。
- `accounts/`：加密账号文件；主密钥保存在系统钥匙串中。
- `--config-dir`、`--data-dir`：分别覆盖配置目录和账号目录。

配置与账号文件通过临时文件替换保存，目录权限为 `0700`，新文件权限为 `0600`。钥匙串被锁定或读取失败时会返回错误，不会生成新密钥覆盖原有数据。

会话过期时先执行 `account refresh`；刷新失败可以重新登录。损坏文件、未知文件版本会返回明确错误。已知账号 ID 的损坏账号可用 `account delete --account ID --yes` 删除本地记录，然后重新登录。

家庭、设备接口目前只请求一个由 `--limit` 控制的结果集。服务器返回 `has_more=true` 时，工具明确提示截断并保留游标信息；请求侧翻页协议尚未验证，因此不宣称结果完整。当前米家 API 使用已有的中国区地址。

## 开发与验证

```sh
make test
make race
make vet
make check
make fuzz
```

自动测试使用临时目录、模拟钥匙串、假 HTTP Transport 或本地 HTTP 服务，不访问真实账号或设备。`make fuzz` 对响应、认证数据、翻译索引和 URN 各运行一轮短时模糊测试。

本轮已覆盖模拟服务中的完整命令流程；真实扫码、系统钥匙串以及选定设备的读写仍需在实际账号环境验收，不能用模拟测试替代。建议依次验证：登录并重启加载、切换默认账号、共享家庭查询、读取属性，再在明确选定的测试设备上验证修改、动作和场景。
