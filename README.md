# ccux

tmux の複数ペインで横断的に動いている **Claude Code セッション**を一覧・プレビューし、選んだセッションが開いている tmux ペインへ一発で移動するツール。[ghux](https://github.com/2357gi/ghux) の Claude Code 版。

```
$ ccux
Claude Sessions > █
  ▲ Waiting   web-app          main          Add OAuth login flow
  ▲ Waiting   api-server       develop  0%   Investigate request timeout
  … Working   dotfiles         master  11.7% Build ccux tool
  … Working   cli-tool         main    14.6% Refactor config loader
                                              ┌─ preview ────────────────────────┐
                                              │ … Working  cli-tool            │
                                              │ branch main                       │
                                              │ ▌ Refactor config loader          │
                                              │ you › Split the loader in two?    │
                                              │ claude › Done — all tests pass.   │
                                              │ ──── live pane ────               │
                                              └───────────────────────────────────┘
```

## できること

- 全 tmux ペインを走査し、`claude` が動いているペインだけを抽出
- 各セッションの **ステータス**（`Waiting` 承認待ち / `Working` 実行中 / `Idle` 入力待ち）と **context 残量 %** を表示
- **recap** をプレビュー表示: AI 生成タイトル・直近のプロンプト・直近の応答・ライブのペイン内容
- 選択すると、そのセッションの tmux ペインへ `switch-client` / `select-window` / `select-pane` で移動
- 注意が必要なもの順（Waiting → Working → Idle）にソート

## インストール

```sh
go install github.com/2357gi/ccux/cmd/ccux@latest
# または、このリポジトリで
make install        # go install ./cmd/ccux ( -> $GOPATH/bin )
```

`fzf` と `tmux` が必要です。

## 使い方

```sh
ccux                 # fzf で対話選択 → 選んだペインへ移動
ccux list            # セッション一覧を標準出力に表示
ccux preview <pane>  # 1ペイン分の status + recap を表示（fzf の preview 用）
ccux jump <pane>     # 指定ペインへ移動
ccux version
```

### tmux からポップアップで呼ぶ

`~/.tmux.conf`:

```tmux
bind-key -T prefix C-j display-popup -w 90% -h 80% -E "exec ccux"
```

`prefix C-j` でフローティングポップアップが開き、選択するとそのペインへ飛びます。

## 仕組み

- **ペイン⇔セッションの対応**: 各 `claude` プロセスの環境変数 `TMUX_PANE`（例 `%9`）を `ps -E` から読み、tmux ペインと一意に対応づける。`claude` プロセスの検出は `ps -axo comm`（`pgrep` は取りこぼすことがある）。
- **ステータス**: `tmux capture-pane` の内容からスピナー / 承認プロンプト / 空の入力欄を判定。
- **recap**: cwd から `~/.claude/projects/<エンコード名>` のトランスクリプト (`*.jsonl`) を特定し、`ai-title` / 最後の user プロンプト / 最後の assistant 応答を抽出。
- 同一 cwd に複数の claude がいる場合は、ペインの最終アクティビティ時刻とトランスクリプトの更新時刻で対応づける（ステータスと移動は常に正確、recap のみベストエフォート）。

## 開発

```sh
make test    # go test ./...
make build   # ./ccux を生成
```

TDD で開発。純粋関数（パース・判定・対応づけ）にロジックを寄せ、各 `internal/*` パッケージに単体テストを置いている。
