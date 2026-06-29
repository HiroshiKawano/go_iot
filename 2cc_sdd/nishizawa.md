# 気象学分野におけるトレンドの検出について

**発表者**: 西澤誠也（神戸大学大学院理学研究科）  
**発表**: CPSセミナー（2008年10月7日）  
**原典**: `nishizawa.pdf`（PowerPoint `CPSセミナー20081007.pptx` を PDF 化、全26スライド）

> 本ファイルは PDF スライドを Markdown に変換したものです。各スライドの本文・数式（LaTeX）・図の内容説明を、原資料のスライド順に収録しています。数式は `$...$` / `$$...$$`（KaTeX/MathJax 記法）です。

---

## 目次

1. 気象学分野におけるトレンドの検出について
2. はじめに
3. 平均値のトレンド
4. トレンドの検出
5. 線形回帰
6. a の推定値 â の期待値と分散
7. â の分布
8. 大気内部変動の分布
9. a_s の分布関数の Edgeworth 展開
10. トレンドの検定
11. Mann-Kendall法
12. Mann-Kendall検定の統計量S・分散・統計量Z
13. Sen's slope
14. Boot Strap 法
15. レアイベントのトレンド
16. 極端現象のトレンドの検出
17. レアイベントのトレンド検出の難しさ
18. イベント出現のバイナリ時系列と統計量 S
19. φの確率密度関数・特性関数とSの特性関数（exp(-ilω/2)展開）
20. Sの確率密度関数および分布関数
21. pについて
22. 成層圏気温への適用
23. 日々気温（北極）の例: NCEP/NCAR（前半期間 vs 後半期間）
24. 月最高気温と極端な高温イベントのトレンド
25. まとめ
26. 冬季極域成層圏の極端な高温イベント

---

## スライド 1 — 気象学分野におけるトレンドの検出について

- 西澤誠也
- 神戸大学大学院理学研究科

---

## スライド 2 — はじめに

- 気候変動
  - 対流圏温暖化
  - 成層圏寒冷化
  - オゾン減少
    - 人間活動が影響

トレンドの値の見積りは重要である

> 📊 **図 (a) Lower troposphere and surface**: 縦軸が「Global anomaly (℃) relative to 1979 to 1990」、横軸が「Year」（およそ1958〜2001年）の時系列折れ線グラフ。凡例は MSU 2LT、UKMO 2LT、Surface の3系列。下層対流圏・地表の全球気温偏差を示し、1960〜1970年代は概ね -0.5℃前後の負偏差、1980年代以降に上昇傾向を示し、1998年付近で約 +0.6℃のピーク。図中の小さな挿入図（縦軸 -0.5〜0.5℃）は「Surface minus Lower Troposphere」（地表と下層対流圏の差）を示し、太い平滑線を重ねて0付近で変動。

> 📊 **図 (b) Lower stratosphere**: 縦軸が「Global anomaly (℃) relative to 1979 to 1990」（-6〜+2℃程度）、横軸が「Year」（およそ1958〜2001年）の時系列折れ線グラフ。凡例は MSU 4、UKMO 4、SSU15X の3系列。下部成層圏の全球気温偏差を示し、全体として寒冷化（低下）傾向。火山噴火イベント Agung（約1963年）、El Chichon（約1982年）、Pinatubo（約1991年）の位置に上向き矢印が付され、噴火直後に一時的な昇温ピーク（El Chichon・Pinatubo で約 +1℃）が見られる。図中の小さな挿入図（縦軸 -0.5〜0.5℃）は「MSU 4 minus UKMO 4」を示し、太い平滑線を重ねて0付近で変動。

---

## スライド 3 — 平均値のトレンド

- トレンドの検出を困難にする要因
  - 内部変動
  - 自己相関
  - データの質のギャップ
    - 観測機器の変化
  - 間欠的な外部強制
    - 火山噴火
  - 長周期外部変動
    - 太陽活動
    - 海洋十年規模変動

---

## スライド 4 — トレンドの検出

- 線形回帰 (最小自乗法)
  - Student's t検定
  - Edgeworth展開を用いた検定 (Nishizawa and Yoden, 2005)
- Mann-Kendall法 ＆ Sen's slope
  - non-parametoric
- Bootstrap法
  - resampling法

---

## スライド 5 — 線形回帰

- 線形回帰モデル (Weatherhead,1998)

$$X_n = an + b + v_n, \quad (n = 1, \cdots, N)$$

$$v_n = \lambda v_{n-1} + \epsilon_n$$

$$b = \begin{cases} b_0, & n < N_g \\ b_0 + \delta, & n \geq N_g \end{cases}$$

- a : トレンド
- λ : 自己相関係数
- ε : ランダム変数
- $b_0$ : 定数
- δ : ギャップ

---

## スライド 6 — a の推定値 â の期待値と分散

- a の推定値, $\hat{a}$, の期待値

$$E[\hat{a}] = a + \frac{h_1 h_5 - h_2 h_4}{h_1 h_3 - h_2^2}\delta$$

- $\hat{a}$ の分散

$$
\begin{aligned}
Var(\hat{a}) &= Var(\epsilon)\frac{h_6(h_1 h_6 - h_4 - 2)}{(h_1 h_6 - h_4^2)(h_3 h_6 - h_5^2) - (h_2 h_6 - h_4 h_5)^2} \\
&\approx Var(\epsilon)\frac{12}{N^3}\frac{1}{(1-\lambda)^2}\frac{1}{\{1 - 3\gamma(1-\gamma)\}} \\
&= Var(v)\frac{12}{N^3}\frac{1+\lambda}{1-\lambda}\frac{1}{\{1 - 3\gamma(1-\gamma)\}}
\end{aligned}
$$

$$\gamma = \frac{N_g - 1}{N}$$

$$
\begin{aligned}
h_1 &= (N-1)(1-\lambda)^2 - (1-\lambda^2), \\
h_2 &= (1-\lambda)\{N(N-1)(1-\lambda)/2 + N + \lambda\}, \\
h_3 &= N(N+1)(2N+1)(1-\lambda)^2/6 + N^2\lambda(1-\lambda) + N\lambda - \lambda^2, \\
h_4 &= (N - N_g)(1-\lambda)^2 + (1-\lambda), \\
h_5 &= (N - N_g)(1-\lambda)\{(N + N_g)(1-\lambda) + 1 + \lambda\}/2 + N_g - (N_g - 1)\lambda, \\
h_6 &= (N - N_g)(1-\lambda)^2 + 1
\end{aligned}
$$

---

## スライド 7 — â の分布

- $\hat{a}$ の分布
  - $\varepsilon$ の分布が正規分布の場合、$\hat{a}$ の分布も正規分布となる
    - 平均と分散が分かれば分布関数が決まる
  - $\lambda=0, \delta=0$ の場合、任意の $\varepsilon$ の分布に対する $\hat{a}-a$ の分布を、Edgeworth展開により求めることができる
    - Nishizawa and Yoden (2005)

---

## スライド 8 — 大気内部変動の分布

- 大気内部変動の分布
  - 降水量： ガンマ分布
    - Wilks and Eggleston (1992)
  - 風速： ワイブル分布
    - Conradsen et al. (1984)
  - 冬季極域成層圏気温： 大きな歪度
    - Yoden et al. (2002)

---

## スライド 9 — a_s の分布関数の Edgeworth 展開

- $a_s\,(=(\hat{a}-a)/\mathrm{Var}(\hat{a})^{1/2})$ の分布関数の Edgeworth 展開

$$F_{a_s}(x) = \Phi(x) + \sum_{l=1}^{\infty} Q_l(x)\phi(x)N^{-l/2}$$

$$Q_{2m+1}(x) = 0,\quad (m = 0, 1, 2, \cdots),$$

$$Q_2(x) = -\frac{3}{40}\frac{\kappa_4}{\kappa_2^2}H_3(x),$$

$$Q_4(x) = -\frac{3}{560}\frac{\kappa_6}{\kappa_2^3}H_5(x) - \frac{9}{3200}\frac{\kappa_4^2}{\kappa_2^4}H_7(x)$$

- $\Phi(x)$: 標準正規分布の分布関数
- $\phi(x)$: 標準正規分布の確率密度関数
- $\kappa_k$: $\varepsilon$ の $k$ 次のキュムラント
- $H_k(x)$: $k$ 次のエルミート多項式

---

## スライド 10 — トレンドの検定

- トレンドの検定
  - トレンドが無い($a=0$)と仮定し、$\hat{a}$ がデータから見積もった値になる確率が十分に小さい場合に、トレンドが存在するとする （無帰仮説検定）
  1. Student's t 検定
     - $\hat{a}$ の分布が正規分布（$=\varepsilon$ の分布が正規分布）である必要
  2. Nishizawa and Yoden (2005)
     - $\lambda=0, \delta=0$ の場合
     - Edgeworth 展開により一般の場合の $\hat{a}$ の分布
     - $\varepsilon$ の高次のモーメントが必要

---

## スライド 11 — Mann-Kendall法

- Kendall (1938)
- ノンパラメトリック法
  - 分布の形によらない
  - 検出力はパラメトリック法に比べて低いことが多い

---

## スライド 12 — Mann-Kendall検定の統計量S・分散・統計量Z

- 統計量 S

$$S = \sum_{n=1}^{N-1} \sum_{m=n+1}^{N} \mathrm{sign}(X_n - X_m) \qquad \mathrm{sign}(x) = \begin{cases} 1 & \text{if} \quad x > 0 \\ 0 & \text{if} \quad x = 0 \\ -1 & \text{if} \quad x < 0 \end{cases}$$

- S の分散

$$Var(X) = \frac{1}{18}\left[ N(N-1)(2N+5) - \sum_i \{ t_i(t_i-1)(2t_i+5) \} \right]$$

t は同じ値の組のデータ数、i はそれぞれの値のグループを表すインデックス

- 統計量 Z

$$Z = \begin{cases} \dfrac{S-1}{\sqrt{Var(S)}} & \text{if} \quad S > 0 \\[2mm] 0 & \text{if} \quad S = 0 \\[2mm] \dfrac{S+1}{\sqrt{Var(S)}} & \text{if} \quad S < 0 \end{cases}$$

- Z の分布関数
  - 標準正規分布になる

---

## スライド 13 — Sen's slope

- Sen's slope
  - トレンドの値の推定値
  - Mann-Kendall検定とともに使われることが多い
    - $Y_{nm}$ の中央値

$$Y_{nm} = \frac{X_n - X_m}{n - m}, \quad n = 2, 3, \cdots, N, \quad m = 1, 2, \cdots, n - 1$$

---

## スライド 14 — Boot Strap 法

- リサンプリング法
  1. N個のデータから無作為復元抽出でN個取り出し、新たな時系列を作る
  2. 新たな時系列からトレンドを見積もる
  3. 1,2,をB回（ブートストラップ反復回数）繰り返す
  4. B個の見積もられたトレンドから経験分布関数を作成する
  5. その分布関数をもとに、元のデータから見積もられたトレンドの無帰仮説検定を行う

---

## スライド 15 — レアイベントのトレンド

- 極端現象トレンド
  - 豪雨
    - Iwashima and Yamamoto (1993)
    - Frei and Schar (2001)
    - Osborn and Hulme (2002)
    - Palmer and Ralsanen (2002)
  - 強い温帯低気圧
    - Graham and Diaz (2001)
  - 強いハリケーン
    - Landsea et al. (1996)

---

## スライド 16 — 極端現象のトレンドの検出

- 極端現象のトレンドの検出
  - 極端現象の日数の経年変化
    - 真夏日、熱帯夜、無降水日等
    - ある閾値を超える現象
  - 極端現象を記録した地点の経年変化（山元他2004）
    - 各地点毎の上位に入る現象

年に数回起こる現象や多地点のデータなど、
各年毎に値を持つ時系列の場合は、
従来のトレンド解析を適用することができる

---

## スライド 17 — レアイベントのトレンド検出の難しさ

- 数年に一度しか起こらないレアイベントのトレンドを検出することは難しい
  - e.g. 成層圏突然昇温
  - エルニーニョ・ラニーニャ
  - 前半・後半に分け、イベントの個数を比較
    - 検出力がとても低い
  - タイムスライス比較実験
    - 数値実験に限定

---

## スライド 18 — イベント出現のバイナリ時系列と統計量 S

- イベントの出現の有無を表すバイナリ時系列

$$\phi_n = \begin{cases} 1 & \text{if the event occurs at } n \\ 0 & \text{if the event does not occur at } n \end{cases}$$

- 統計量 S
  - イベントが起こった時系列の位置（全時刻の平均を引いた）の総和
  - 前半（後半）にイベントが多いと負（正）

$$S = \sum_{n=1}^{N} \left( n - \frac{N+1}{2} \right) \phi_n$$

---

## スライド 19 — φの確率密度関数・特性関数とSの特性関数（exp(-ilω/2)展開）

- φの確率密度関数および特性関数

$$f_\phi(x) = (1-p)\delta(x) + p\delta(x-1)$$

$$\psi_\phi(\omega) = (1-p) + pe^{-i\omega}$$

  pはイベントが起こる確率

- Sの特性関数

$$\psi_S(\omega) = \prod_{n=1}^{N} \psi_\phi\left(\left(n - \frac{N+1}{2}\right)\omega\right)$$

$$= (1-p)^N \prod_{n=1}^{N} \left\{ 1 + A e^{-i\left(n - \frac{N+1}{2}\right)\omega} \right\}$$

  $A = p/(1-p)$

- exp(-ilω/2)で展開

$$\sum_{l=-\infty}^{\infty} Q_{l/2,N}\, e^{-i\frac{l}{2}\omega} = \prod_{n=1}^{N} \left\{ 1 + A e^{-i\left(n - \frac{N+1}{2}\right)\omega} \right\}$$

---

## スライド 20 — Sの確率密度関数および分布関数

- Sの確率密度関数および分布関数

$$f_S(x) = (1-p)^N \sum_{l=-\infty}^{\infty} Q_{l/2,N}\, \delta\left(x - l/2\right)$$

$$F_S(x) = (1-p)^N \sum_{l=-\infty}^{\lfloor x \rfloor} Q_{l/2,N}$$

- Q は漸化式

$$Q_{l/2,N+2} = (1+A^2)Q_{l/2,N} + A Q_{(l/2)+(N+1)/2,\,N} + A Q_{(l/2)-(N+1)/2,\,N}$$

$$Q_{l/2,1} = \begin{cases} 1+A & \text{for} \quad l = 0 \\ 0 & \text{for} \quad l \neq 0 \end{cases}$$

$$Q_{l/2,2} = \begin{cases} 1+A^2 & \text{for} \quad l = 0 \\ A & \text{for} \quad l = \pm 1 \\ 0 & \text{for} \quad \text{other } l \end{cases}$$

- 以下を満たす

$$Q_{-l/2,N} = Q_{l/2,N}$$

$$Q_{l/2,N} = 0 \quad \text{for} \quad |l| > l_{\max}$$

$$l_{\max} = \begin{cases} N^2/4 & \text{for even } N \\ (N^2-1)/4 & \text{for odd } N \end{cases}$$

---

## スライド 21 — pについて

- pについて
  - 多くの場合pは未知数である
  - $M(=\Sigma\phi)/N$ であると仮定する
    - 最尤推定値
  - $f_S(x)$ はpの不確実性の分広がる

---

## スライド 22 — 成層圏気温への適用

- 成層圏の平均気温は寒冷化
  - 温室効果ガスの増加
  - オゾンの減少

- 成層圏突然昇温の存在
  - 冬季極域成層圏気温が、数日で50度程度上がることも

---

## スライド 23 — 日々気温（北極）の例: NCEP/NCAR（前半期間 vs 後半期間）

- daily temperature (North Pole)  (NCEP/NCAR)
  - <span style="color:red">1979/1980 – 1992/1993</span>
  - <span style="color:blue">1993/1994 – 2005/2006</span>

> 📊 **図**: 北極・10hPa の日々気温（K）の時系列。横軸は「Days since 1 December」（0〜120日）、縦軸は気温で約190〜270K（目盛は200, 220, 240, 260）。各冬の系列を重ね描きしており、赤の実線が前半期間（1979/1980〜1992/1993）、青の破線が後半期間（1993/1994〜2005/2006）。太い赤実線・太い青破線はそれぞれの平均を示す。ベースは初冬で約200K前後だが、突然昇温（成層圏突然昇温）に伴い240〜270K近くまでスパイク状に上昇する系列が多数見られ、季節進行とともに全体の平均も右肩上がりに上昇する。図中には、前半期間（赤）で60〜100日付近の高温スパイク群を囲む赤い楕円と、後半期間（青）で0〜40日付近の早い時期の高温イベントを囲む青い楕円が描かれ、突然昇温の発生時期の違いを強調している。

---

## スライド 24 — 月最高気温と極端な高温イベントのトレンド

- 月最高気温
  - 寒冷化トレンド
    - Feb　10hPa
    - Mar　10,30hPa
- 極端な高温イベント
  - 27年平均＋1σ
  - 増加トレンド
    - Dec　10hPa (91%)
    - Jan　30hPa (94%)
  - 減少トレンド
    - Feb　10,30hPa (99,94%)
    - Mar　10,30hPa (97,97%)

> 📊 **図**: Dec・Jan・Feb・Mar の各月について、10hPa と 30hPa の月最高気温の時系列を縦に並べた8段（4月×2気圧面）の折れ線グラフ。横軸は year（おおむね1980〜2005/2006年）、縦軸は気温で約200〜250前後（目盛は200・250）。各パネルとも実線（月最高気温）と破線（もう一方の系列、月平均ないし基準値とみられる）が描かれ、水平の細い基準線（27年平均＋1σの閾値とみられる）が引かれている。閾値を上回る年には青い丸（○）マーカー、特定の年には赤い×マーカーが付され、極端な高温イベントの発生年を示している。右端ラベルは上から Dec(10hPa/30hPa)、Jan(10hPa/30hPa)、Feb(10hPa/30hPa)、Mar(10hPa/30hPa)。

---

## スライド 25 — まとめ

- 平均のトレンド
  - 線形回帰
    - Student's t検定
    - Edgeworth展開による検定
  - Mann-Kendall検定 & Sen's slope
  - BootStrap検定
- レアイベントの検定
  - イベント発生位置の総和

---

## スライド 26 — 冬季極域成層圏の極端な高温イベント

- 冬季極域成層圏　極端な高温イベント
  - 12，1月に増加トレンド
  - 1，3月に減少トレンド

  CO$_2$増加により、突然昇温がおこるタイミングが早まる (Inatsu et al, 2007)

---
