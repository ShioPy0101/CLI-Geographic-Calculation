# CLI-Geographic-Calculation

## 実行方法

基本的な使い方
SQL Like な記法で国土データの svg レンダリング結果が要求できます。

```
curl -sS \
  --get \
  --data-urlencode "query=SELECT * FROM rail WHERE company = '四国旅客鉄道' AND line IN ('鳴門線')" \
  https://cli-geographic-calculation.vercel.app/api/2023/rail
```

会社を複数指定する

```
curl -sS \
  --get \
  --data-urlencode "query=SELECT * FROM rail WHERE company IN ('東急電鉄' , '小田急電鉄')"  \
  https://cli-geographic-calculation.vercel.app/api/2023/rail/svg -o tokyu-odakyu.svg
```

会社を複数指定する + 路線を指定する

```
curl -sS   --get   --data-urlencode "query=SELECT * FROM rail WHERE company IN ('東日本旅客鉄道' , '東海旅客鉄道' ) AND line = '中央線'"   https://cli-geographic-calculation.vercel.app/api/2023/rail/svg -o chuo.svg
```

路線選択 DSL で駅間だけを指定する

```
curl -sS \
  --get \
  --data-urlencode "query=SELECT 山手線 BETWEEN \"新宿\" AND \"東京\", 総武線;" \
  https://cli-geographic-calculation.vercel.app/api/2023/rail/svg -o route-section.svg
```

`路線名` のみを指定した場合は全線、`路線名 BETWEEN 始点駅 AND 終点駅` を指定した場合はその駅間だけを描画対象にします。

実路線形状 GeoJSON を使って描画する

```
curl -sS \
  --get \
  --data-urlencode "query=SELECT 山陽線 BETWEEN 姫路 AND 神戸, 東海道線 BETWEEN 神戸 AND 大阪 OPTION geographic, single_line;" \
  https://cli-geographic-calculation.vercel.app/api/2023/rail/svg -o route-geographic.svg
```

`OPTION geographic` は `pkg/giodata/N05-24_RailroadSection2.geojson` の LineString を路線形状として使います。`OPTION single_line` と併用すると、連続した複数区間を 1 本の SVG path として出力します。

![sample image](./doc/sample-chuo.png)
出力 svg の例（テスト版）

## 権利情報

### 出典

「国土数値情報（鉄道 データ）」（国土交通省）[国土数値情報（鉄道 データ）](https://nlftp.mlit.go.jp/ksj/gml/datalist/KsjTmplt-N02-2022.html)を加工して作成

## 備考

（任意）
本ソフトウェアを用いて公開した動画の概要欄に「しおまち （ https://twitter.com/ShioPy0101 ） のソフトウェアを用いて地理情報を用いたアニメーションを作成しました」的なことを書いてくれると、うれしいです。
