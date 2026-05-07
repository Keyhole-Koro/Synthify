# Multimedia Processing & Transcription Strategy

## 背景
Synthify ではドキュメントだけでなく、音声・動画・画像などのメディアファイルから知識を抽出するニーズがある。
GCS FUSE を活用した Worker アーキテクチャにおいて、これらのファイルを効率的に処理し、文字起こし（Transcription）や解析を行うための戦略を定義する。

## コスト比較 (2026年5月時点)

1分あたりの処理コスト（入力のみ）の概算：

| 手法 | 音声 (1分) | 動画+音声 (1分) | 備考 |
| :--- | :--- | :--- | :--- |
| **Gemini 1.5 Flash** | **~$0.00014** | **~$0.00131** | 圧倒的に安価。1FPSサンプリング。 |
| **Gemini 1.5 Pro** | ~$0.0024 | ~$0.0218 | 高精度。1FPSサンプリング。 |
| **Google Cloud STT (V2)** | $0.016 | - | 文字起こし専用。バッチなら $0.003。 |
| **Self-hosted Whisper** | **~$0.0005** | - | Cloud Run L4 GPU 利用。大量処理で最安。 |

### 比較の結論
- **短期・低〜中容量**: **Gemini 1.5 Flash** が最適。実装が容易（既存の LLM Worker 経路を流用可能）で、かつコストも十分に低い。文字起こしだけでなく「動画の内容要約」なども一括で行える。
- **長期・超高容量**: **Self-hosted Whisper (Cloud Run GPU)** を検討。1時間分の音声を数分（数十円）で処理できるが、インフラ管理コスト（別 Worker の維持）が発生する。

## Gemini 3.1 の動画理解能力と出力仕様

Gemini 3.1 (特に Pro/Flash) は、動画を「視覚情報のシーケンス」と「音声」として同時に理解し、時間軸に沿った高度な推論が可能。

### 主な能力
- **タイムスタンプ付きイベント抽出**: 「02:15 にスライドが切り替わった」等の時間指定付き解析。
- **マルチモーダル融合**: 画面内の文字 (OCR)、音声 (Speech)、視覚的状況 (Visual) を組み合わせて、1つのコンテキストとして理解する。
- **構造化データ出力**: 任意の JSON スキーマを指定して、解析結果をプログラムで扱いやすい形で受け取れる。

### 出力イメージ (JSON)
動画内の重要なシーンを抽出し、チャプター化する場合のレスポンス例：

```json
[
  {
    "start_timestamp": "00:00:00",
    "end_timestamp": "00:01:20",
    "topic": "イントロダクション",
    "summary": "スピーカーが登壇し、本日のアジェンダを紹介。"
  },
  {
    "start_timestamp": "00:01:20",
    "end_timestamp": "00:05:45",
    "topic": "システムデモ",
    "summary": "新機能のライブデモ。音声では『高速な検索能力』を強調している。"
  }
]
```

### テクニカルスペック
- **サンプリング**: デフォルトで 1FPS (1秒間に1フレーム) を処理。
- **トークン消費量**: 1秒あたり合計約 290 トークン程度 (Video 258 + Audio 32)。
- **長尺対応**: 数時間の動画も 1M トークンのコンテキスト窓に収まる範囲で一括処理可能。

## 推奨アーキテクチャ

### フェーズ 1: LLM Worker 統合 (Gemini Flash 活用)
既存の LLM Worker にメディア処理能力を持たせる。

1.  **メディア検知**: `extract_text` ツールが MIME タイプを判定。
2.  **ffmpeg 変換**: 巨大なファイルは `ffmpeg` (Worker 内蔵) で軽量な音声 (.mp3) または画像シーケンスに変換。
3.  **Gemini 解析**: Gemini 1.5 Flash にファイルを渡し、文字起こしと構造化データを一括取得。
4.  **GCS FUSE キャッシュ**: 結果を `/mnt/gcs/.cache/` に保存し、テキストと同様に検索可能にする。

### フェーズ 2: 専用 Transcription Worker (Whisper 活用)
コストが課題になった場合、または Gemini の文字起こし精度では不十分な場合に導入。

1.  **Worker 分離**: Python/PyTorch ベースの GPU 対応 Cloud Run サービスを構築。
2.  **非同期連携**: LLM Worker が専用 Worker の API を叩き、GCS パスを渡して文字起こしを依頼する。

## 実装上の考慮事項

### GCS FUSE の役割
- **ストリーミング処理**: `ffmpeg` に GCS 上の動画を直接食わせることで、メモリ消費（OOM）を抑える。
- **中間生成物の保存**: 抽出した音声ファイルや、一時的な画像フレームの保存場所として活用。

### ffmpeg の組み込み
Worker の Docker イメージに `ffmpeg` を含める。
```dockerfile
RUN apt-get update && apt-get install -y ffmpeg && rm -rf /var/lib/apt/lists/*
```

## 関連ドキュメント
- [worker-gcs-fuse.md](worker-gcs-fuse.md)
- [worker-tools-stub.md](worker-tools-stub.md)
