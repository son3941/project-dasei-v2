package handler

import (
	"math/rand"
	"regexp"
	"strings"
)

type KaibunshoMode string

const (
	KaibunshoZombie      KaibunshoMode = "zombie"
	KaibunshoConspiracy  KaibunshoMode = "conspiracy"
	KaibunshoAITranslate KaibunshoMode = "aimistranslate"
	KaibunshoSpiritual   KaibunshoMode = "spiritual"
	KaibunshoBizGuru     KaibunshoMode = "bizguru"
	KaibunshoMixed       KaibunshoMode = "mixed"
)

var kaibunshoModes = []KaibunshoMode{
	KaibunshoZombie,
	KaibunshoConspiracy,
	KaibunshoAITranslate,
	KaibunshoSpiritual,
	KaibunshoBizGuru,
	KaibunshoMixed,
}

func randomKaibunshoMode() KaibunshoMode {
	return kaibunshoModes[rand.Intn(len(kaibunshoModes))]
}

type KaibunshoSettings struct {
	Mode       KaibunshoMode
	Level      int
	MixRate    int
	ContamRate int
}

func randomKaibunshoSettings() KaibunshoSettings {
	return KaibunshoSettings{
		Mode:       randomKaibunshoMode(),
		Level:      rand.Intn(5) + 1, // 1～5
		MixRate:    rand.Intn(101),   // 0～100
		ContamRate: rand.Intn(101),   // 0～100
	}
}
func pickString(items []string) string {
	return items[rand.Intn(len(items))]
}

func maybe(prob float64) bool {
	return rand.Float64() < prob
}

var kaibunshoSubjects = []string{
	"納豆", "この猫", "靴", "アルゴリズム", "私の祖父", "冷凍ブリトー", "月", "ほうれん草", "このカメ", "経済",
	"ゴールデンレトリバー", "コンクリート", "霧", "ブロックチェーン", "インターネット", "この投稿", "AI", "私の上司",
	"タコ", "洗濯機", "冷蔵庫", "パン粉", "信号機", "エレベーター", "給湯器", "コピー機", "エアコン", "充電器",
	"自動販売機", "監視カメラ", "火災報知器", "郵便ポスト", "街灯", "電柱", "マンホール", "踏切", "横断歩道",
	"私の祖母", "隣人", "配達員", "税理士", "歯医者", "消防士", "管理組合", "町内会長", "回覧板",
	"スズメ", "カラス", "ハト", "イルカ", "クマ", "アリ", "ミミズ", "カタツムリ", "ゴキブリ", "シャチ",
	"富士山", "東京タワー", "琵琶湖", "北海道", "沖縄", "南極", "月面基地", "深海", "成層圏", "異次元",
	"量子コンピューター", "衛星", "ドローン", "宇宙ステーション", "潜水艦", "気球", "時計台", "図書館",
}
var kaibunshoObjects = []string{
	"税務処理", "量子力学", "民主主義", "第三次元", "軌道計算", "水分補給", "逆物流", "帯域幅", "チーズ製造",
	"確定申告", "宇宙条約", "炭素排出権", "火曜日の概念", "カルマ収支", "オーラ管理", "波動調整",
	"マニフェスト申請", "チャクラ認証", "アカシックレコード", "松果体活性化", "第六感校正",
	"収益化戦略", "スケーラビリティ", "KPI最適化", "パッシブインカム", "マインドセット更新",
	"ネットワーキング処理", "グロースハック", "ピボット判断", "エグジット戦略",
	"人口削減計画", "5G照射プログラム", "マイクロチップ登録", "記憶改ざん処理", "洗脳プロトコル",
	"影の政府通信", "秘密文書転送", "ケムトレイル散布", "フラットアース測量", "爬虫類人認証",
	"感情スペクトル", "行動パターン", "生体認証", "熱量変換", "燃料補給", "カロリー処理",
	"エネルギー充填", "位相変換", "データ圧縮", "バイナリ変換", "パケット転送", "プロトコル更新",
	"重力調整", "磁場操作", "時空間調整", "次元跳躍", "平行世界接続", "因果律編集",
	"お米の調達", "謎の液体の管理", "ブランケットの権限", "スリッパの運営", "傘の統括",
}
var kaibunshoVerbs = []string{
	"を完了しました", "を開始しました", "を発見しました", "を解放しました", "を起動しました",
	"を達成しました", "を更新しました", "を処理しました", "を展開しました", "を実行しました",
	"を承認しました", "を記録しました", "を検出しました", "を解析しました", "を最適化しました",
	"を破壊しました", "を構築しました", "を監視しています", "を収集しました", "を送信しました",
	"を遮断しました", "を解除しました", "を強化しました", "を再起動しました", "を終了しました",
	"を超えました", "に到達しました", "に移行しました", "に対処しました", "に侵入しました",
	"を拒否しました", "を暗号化しました", "を復号化しました", "を学習しました", "を吸収しました",
}
var kaibunshoModifiers = []string{
	"量子化された", "第三の", "無限の", "緊急の", "謎の", "秘密の", "禁断の", "古代の", "未来の", "並行した",
	"最適化された", "暗号化された", "宇宙規模の", "惑星間の", "次世代の", "廃止予定の", "β版の",
	"認定された", "非公開の", "政府管轄の", "民間委託の", "自動化された", "手動による", "強制的な",
}

func buildKaibunshoTranslation() string {
	subject := pickString(kaibunshoSubjects)

	subject2 := subject
	for subject2 == subject {
		subject2 = pickString(kaibunshoSubjects)
	}

	object := pickString(kaibunshoObjects)
	modifier := pickString(kaibunshoModifiers)
	verb := pickString(kaibunshoVerbs)

	switch rand.Intn(10) {
	case 0:
		return subject + "は" + object + verb
	case 1:
		return subject + "は" + modifier + object + verb
	case 2:
		return subject + "と" + subject2 + "が" + object + verb
	case 3:
		return subject + "による" + object + verb
	case 4:
		return subject + "が" + modifier + object + verb
	case 5:
		return subject + "の" + object + verb
	case 6:
		return subject + "と" + subject2 + "による" + modifier + object + verb
	case 7:
		return subject + "が静かに" + object + verb
	case 8:
		return subject + "が自動的に" + object + verb
	default:
		return subject + "による緊急の" + modifier + object + verb
	}
}

var kaibunshoConspiracySubjects = []string{
	"影の政府", "ディープステート", "ビルゲイツ", "爬虫類人", "フリーメイソン", "ケムトレイル",
	"5Gタワー", "ワクチンマイクロチップ", "NASAの嘘", "製薬会社", "世界経済フォーラム",
	"イルミナティ", "ロスチャイルド家", "ロックフェラー財団", "秘密結社", "影の国際委員会",
	"月面詐欺の黒幕", "フラットアース協会", "古代宇宙人", "マスメディアの操作者",
}

var kaibunshoConspiracyObjects = []string{
	"人口削減計画", "マインドコントロール装置", "水道水への薬物混入", "記憶改ざんプログラム",
	"洗脳電波", "DNAマーカー追跡", "生体監視システム", "思考盗聴装置", "感情操作ガス",
	"都市伝説の書き換え", "歴史の隠蔽工作", "真実の抹消", "内部告発者の排除",
	"次世代支配構造", "新世界秩序の設計", "シオン議定書の更新", "グレートリセット計画",
}

var kaibunshoConspiracyVerbs = []string{
	"をすでに実行中です", "を秘密裏に展開しています", "を隠蔽しています", "を進行させています",
	"を準備しています", "を完了させました", "を黙認しています", "を後援しています",
	"を否定しています", "を妨害しています", "を検閲しています", "を削除しました",
}
var kaibunshoSpiritualSubjects = []string{
	"宇宙エネルギー", "あなたの守護天使", "ハイヤーセルフ", "月のエネルギー", "クリスタルの振動",
	"松果体", "アカシックレコード", "あなたのオーラ", "カルマ", "マニフェストの法則", "第三の目",
	"プレアデス星人", "シリウスの意識", "光の存在", "集合的無意識", "源の意識",
	"クンダリーニエネルギー", "マーキュリー逆行", "満月のエネルギー", "新月の意図",
}

var kaibunshoSpiritualVerbs = []string{
	"があなたに重要なメッセージを送っています",
	"があなたの魂の目覚めを促しています",
	"があなたの波動を444Hzに調整しました",
	"があなたの前世の記憶を解放しました",
	"があなたのチャクラを完全に開きました",
	"があなたとの繋がりを回復しました",
	"があなたのソウルメイトを引き寄せています",
	"が豊かさのエネルギーを解放しました",
	"があなたの次元上昇の準備を完了しました",
	"がエゴを超えた意識を呼び覚ましました",
	"があなたを高次元にアップロードしています",
	"があなたの魂の契約を更新しました",
	"があなたの光のコードを活性化しました",
	"があなたの使命を今思い出させています",
}
var kaibunshoBizSubjects = []string{
	"成功者のマインドセット", "7桁の売上", "私のメンター", "朝4時の習慣", "コンフォートゾーン",
	"パッシブインカム", "ROI", "スケーラビリティ", "グロースハック", "ビジョンボード",
	"ネットワーキング", "メンタルモデル", "レバレッジ", "複利の法則", "エグゼクティブマインド",
	"成功の習慣", "敗者の思考", "勝者のサークル", "富の法則", "億万長者のルーティン",
}

var kaibunshoBizVerbs = []string{
	"が人生を180度変えます",
	"が月収を10倍にします",
	"が敗者の思考を排除します",
	"が億万長者への扉を開きます",
	"が成功の公式を解読しました",
	"が勝者を選別します",
	"がコンフォートゾーンを破壊します",
	"がゲームのルールを変えます",
	"が富を引き寄せます",
	"がスケールの限界を突破します",
	"が収益化を自動化します",
	"が市場を支配します",
	"が99%には理解できません",
	"が選ばれし者を認定します",
}
var kaibunshoAIWrongInputs = []string{
	"あなたの入力",
	"検出されたテキスト",
	"処理されたデータ",
	"解析されたフレーズ",
	"言語モデルの解釈",
	"ニューラルネットの出力",
	"セマンティック解析結果",
	"トークン列",
}

var kaibunshoAIWrongTranslations = []string{
	"馬が空を高速飛行しています",
	"祖母がWi-Fiを美味しく食べています",
	"冷蔵庫が市議会選挙に出馬しました",
	"月が確定申告を電子提出しました",
	"お茶が量子力学の論文を発表しました",
	"猫がビットコインをフルノードでマイニング中です",
	"信号機が深夜に恋愛相談を開始しました",
	"パン粉が民主共和国を統治しています",
	"タコが公認会計士として独立開業しました",
	"ゴミ箱がカントの哲学を独学で習得中",
	"傘が国際連合の本会議で演説しました",
	"スリッパが宇宙開発の予算を承認しました",
	"電柱が恋に落ちて詩を書いています",
	"郵便ポストが仮想通貨を発行しました",
	"エレベーターが禅の悟りを開きました",
	"自動販売機が政党を結成しました",
	"洗濯機が映画監督としてデビューしました",
	"コピー機が宇宙人と交渉中です",
	"給湯器が株式市場を予測しています",
	"信号機が量子もつれを観測しました",
	"踏切がAIガバナンスを提唱しています",
	"マンホールが国境問題を調停しました",
}

func buildKaibunshoModeTranslation(mode KaibunshoMode, postText string) string {
	switch mode {
	case KaibunshoZombie:
		return buildKaibunshoTranslation()

	case KaibunshoConspiracy:
		s := pickString(kaibunshoConspiracySubjects)
		o := pickString(kaibunshoConspiracyObjects)
		v := pickString(kaibunshoConspiracyVerbs)

		if maybe(0.4) {
			s2 := pickString(kaibunshoConspiracySubjects)
			return s + "と" + s2 + "が" + o + v
		}
		return s + "が" + o + v

	case KaibunshoAITranslate:
		wrong := pickString(kaibunshoAIWrongTranslations)

		if postText != "" {
			runes := []rune(postText)
			if len(runes) > 15 {
				runes = runes[:15]
			}
			return "「" + string(runes) + "」は「" + wrong + "」を意味します"
		}

		input := pickString(kaibunshoAIWrongInputs)
		return input + "の正確な翻訳結果：「" + wrong + "」"

	case KaibunshoSpiritual:
		s := pickString(kaibunshoSpiritualSubjects)
		v := pickString(kaibunshoSpiritualVerbs)
		return s + v

	case KaibunshoBizGuru:
		s := pickString(kaibunshoBizSubjects)
		v := pickString(kaibunshoBizVerbs)
		return s + v

	case KaibunshoMixed:
		return buildKaibunshoMixedTranslation()
	}

	return buildKaibunshoTranslation()
}
func buildKaibunshoMixedTranslation() string {
	all := []string{}

	all = append(all, kaibunshoSubjects...)
	all = append(all, kaibunshoObjects...)
	all = append(all, kaibunshoConspiracySubjects...)
	all = append(all, kaibunshoConspiracyObjects...)
	all = append(all, kaibunshoSpiritualSubjects...)
	all = append(all, kaibunshoBizSubjects...)

	a := pickString(all)

	bCandidates := make([]string, 0, len(all)-1)
	for _, item := range all {
		if item != a {
			bCandidates = append(bCandidates, item)
		}
	}
	b := pickString(bCandidates)

	verbs := []string{}
	verbs = append(verbs, kaibunshoVerbs...)
	verbs = append(verbs, kaibunshoConspiracyVerbs...)
	verbs = append(verbs, kaibunshoSpiritualVerbs...)
	verbs = append(verbs, kaibunshoBizVerbs...)

	v := pickString(verbs)

	return a + "は" + b + v
}
func extractKaibunshoWords(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	found := make(map[string]struct{})

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`[一-龯々〆〤ヵヶ]{2,}`),
		regexp.MustCompile(`[ァ-ヶーｦ-ﾟ]{2,}`),
		regexp.MustCompile(`[ぁ-ん]{3,}`),
		regexp.MustCompile(`[a-zA-Z]{3,}`),
	}

	for _, pattern := range patterns {
		for _, match := range pattern.FindAllString(text, -1) {
			found[match] = struct{}{}
		}
	}

	splitter := regexp.MustCompile(`[\s　、。！？!?・…「」【】（）()\r\n]+`)
	for _, part := range splitter.Split(text, -1) {
		part = strings.TrimSpace(part)

		runeLen := len([]rune(part))
		if runeLen >= 2 && runeLen <= 14 {
			found[part] = struct{}{}
		}
	}

	words := make([]string, 0, len(found))
	for word := range found {
		if len([]rune(word)) >= 2 {
			words = append(words, word)
		}
	}

	return words
}

var kaibunshoSentenceTemplates = []func(string, string) string{
	func(a, b string) string { return a + "と思ったのは私だけだろうか" },
	func(a, b string) string { return "なぜ" + a + "なのか考えてほしい" },
	func(a, b string) string { return a + "。偶然では説明できない" },
	func(a, b string) string { return a + "は" + b + "につながっている" },
	func(a, b string) string { return a + "の真実を知った時、私は震えた" },
	func(a, b string) string {
		return "これを読んでいるあなたも" + a + "を感じているはずだ"
	},
	func(a, b string) string { return a + "について、誰も語ろうとしない" },
	func(a, b string) string { return a + "から" + b + "が始まる" },
	func(a, b string) string { return a + "。点と点が繋がった" },
	func(a, b string) string { return a + "だということに気づいてしまった" },
	func(a, b string) string { return a + "と" + b + "は同じ組織が管理している" },
	func(a, b string) string { return a + "を知ってから人生が変わった" },
}
var kaibunshoSentenceTemplates2 = []func(string, string) string{
	func(a, b string) string { return a + "は削除される前に見てください" },
	func(a, b string) string { return a + "が" + b + "を隠している理由がわかった" },
	func(a, b string) string { return "私だけが" + a + "に気づいている" },
	func(a, b string) string { return a + "の影響で世界は変わる" },
	func(a, b string) string { return a + "と" + b + "の関係を調べてみてください" },
	func(a, b string) string { return a + "。これが真実です" },
	func(a, b string) string { return "なぜか" + a + "だけが不自然に見える" },
	func(a, b string) string { return a + "を疑うことから始めてほしい" },
	func(a, b string) string { return a + "は" + b + "の隠喩だった" },
	func(a, b string) string { return a + "が世界を支配している" },
	func(a, b string) string { return a + "を" + b + "に変換すると真実が見えてくる" },
	func(a, b string) string { return a + "の裏に潜む陰謀に気づいてしまった" },
	func(a, b string) string { return a + "さえ理解できれば全てがわかる" },
}

func pickKaibunshoSentenceTemplate() func(string, string) string {
	all := make([]func(string, string) string, 0,
		len(kaibunshoSentenceTemplates)+len(kaibunshoSentenceTemplates2))

	all = append(all, kaibunshoSentenceTemplates...)
	all = append(all, kaibunshoSentenceTemplates2...)

	return all[rand.Intn(len(all))]
}
func buildKaibunshoSentenceWithInjection(
	injectedWords []string,
	mixRate int,
	communityWords []string,
	contamRate int,
) string {
	pool := make([]string, 0, len(kaibunshoSubjects)+len(kaibunshoObjects))
	pool = append(pool, kaibunshoSubjects...)
	pool = append(pool, kaibunshoObjects...)

	pickSlot := func(exclude string) string {
		r := rand.Intn(100)

		if len(communityWords) > 0 && r < contamRate {
			candidates := filterKaibunshoWords(communityWords, exclude)
			if len(candidates) > 0 {
				return pickString(candidates)
			}
		}

		if len(injectedWords) > 0 && r < contamRate+mixRate {
			candidates := filterKaibunshoWords(injectedWords, exclude)
			if len(candidates) > 0 {
				return pickString(candidates)
			}
		}

		candidates := filterKaibunshoWords(pool, exclude)
		return pickString(candidates)
	}

	a := pickSlot("")
	b := pickSlot(a)

	return pickKaibunshoSentenceTemplate()(a, b)
}
func filterKaibunshoWords(words []string, exclude string) []string {
	if exclude == "" {
		return words
	}

	result := make([]string, 0, len(words))
	for _, word := range words {
		if word != exclude {
			result = append(result, word)
		}
	}

	return result
}
func buildKaibunshoTemplateSentences(
	level int,
	injectedWords []string,
	mixRate int,
	communityWords []string,
	contamRate int,
) []string {
	if level <= 0 {
		return nil
	}

	result := make([]string, 0, level)

	for i := 0; i < level; i++ {
		result = append(result,
			buildKaibunshoSentenceWithInjection(
				injectedWords,
				mixRate,
				communityWords,
				contamRate,
			),
		)
	}

	return result
}
func buildKaibunshoDirectInjectionLines(
	postText string,
	mixRate int,
) []string {
	postText = strings.TrimSpace(postText)

	if postText == "" || mixRate < 40 {
		return nil
	}

	splitter := regexp.MustCompile(`[\n。！？!?]+`)
	rawFragments := splitter.Split(postText, -1)

	fragments := make([]string, 0, len(rawFragments))

	for _, fragment := range rawFragments {
		fragment = strings.TrimSpace(fragment)

		if len([]rune(fragment)) >= 2 {
			fragments = append(fragments, fragment)
		}
	}

	if len(fragments) == 0 {
		return nil
	}

	if mixRate >= 70 {
		limit := 3
		if len(fragments) < limit {
			limit = len(fragments)
		}

		result := make([]string, 0, limit)

		for _, fragment := range fragments[:limit] {
			result = append(result, fragment+"。")
		}

		return result
	}

	if rand.Intn(100) < mixRate {
		return []string{fragments[0] + "。"}
	}

	return nil
}

type KaibunshoResult struct {
	Text       string
	Mode       KaibunshoMode
	Level      int
	MixRate    int
	ContamRate int
}

func makeKaibunsho(
	postText string,
	communityWords []string,
) KaibunshoResult {
	settings := randomKaibunshoSettings()

	injectedWords := extractKaibunshoWords(postText)

	translation := buildKaibunshoModeTranslation(
		settings.Mode,
		postText,
	)

	directLines := buildKaibunshoDirectInjectionLines(
		postText,
		settings.MixRate,
	)

	extraLines := buildKaibunshoTemplateSentences(
		settings.Level-1,
		injectedWords,
		settings.MixRate,
		communityWords,
		settings.ContamRate,
	)

	parts := make([]string, 0)

	parts = append(parts, directLines...)

	if translation != "" {
		parts = append(parts, translation)
	}

	parts = append(parts, extraLines...)

	text := strings.Join(parts, "\n")

	if len(communityWords) > 0 {
		hasCommunityWord := false

		for _, word := range communityWords {
			word = strings.TrimSpace(word)
			if word != "" && strings.Contains(text, word) {
				hasCommunityWord = true
				break
			}
		}

		if !hasCommunityWord {
			word := strings.TrimSpace(
				communityWords[rand.Intn(len(communityWords))],
			)

			if word != "" {
				text = strings.TrimSpace(text + "\n" + word)
			}
		}
	}
	return KaibunshoResult{
		Text:       text,
		Mode:       settings.Mode,
		Level:      settings.Level,
		MixRate:    settings.MixRate,
		ContamRate: settings.ContamRate,
	}
}
func limitKaibunshoLength(text string) string {
	text = strings.TrimSpace(text)

	runes := []rune(text)
	if len(runes) <= 140 {
		return text
	}

	return strings.TrimSpace(string(runes[:140]))
}
