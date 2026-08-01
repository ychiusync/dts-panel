package cluster

import "strings"

// WorldParam 单个世界参数定义
type WorldParam struct {
	Key       string
	Label     string
	Category  string
	Type      string   // select, toggle, number
	Options   []string // select/toggle 类型的选项
	Default   string
}

// ParamCategory 参数分类
type ParamCategory struct {
	Key     string
	Label   string
	Icon    string
	Params  []WorldParam
}

// 通用选项
var commonSelectOptions = []string{
	"default",
	"none",
	"low",
	"medium",
	"high",
	"very_high",
	"always",
	"never",
	"more",
	"more_more",
	"less",
	"less_less",
	"twice_as_much",
	"half_as_much",
}

var toggleOptions = []string{"false", "true"}

// 地面世界参数分类
var masterParamCategories = []ParamCategory{
	{
		Key: "behavior", Label: "行为", Icon: "🎮",
		Params: []WorldParam{
			{"day", "白天长度", "behavior", "select", []string{"default", "very_short", "short", "medium", "long", "very_long", "maximum"}, "default"},
			{"night", "夜晚长度", "behavior", "select", []string{"default", "very_short", "short", "medium", "long", "very_long", "maximum", "none"}, "default"},
			{"resettime", "重置时间", "behavior", "select", []string{"default", "none", "60", "300", "900", "1800", "3600"}, "default"},
			{"has_ocean", "开启海洋", "behavior", "toggle", toggleOptions, "true"},
			{"wanderingtrader_enabled", "流浪商人", "behavior", "toggle", toggleOptions, "true"},
			{"no_joining_islands", "不加入孤岛", "behavior", "toggle", toggleOptions, "true"},
			{"no_wormholes_to_disconnected_tiles", "禁用孤立岛虫洞", "behavior", "toggle", toggleOptions, "true"},
			{"keep_disconnected_tiles", "保留不连通地块", "behavior", "toggle", toggleOptions, "true"},
			{"spawnmode", "出生点模式", "behavior", "select", []string{"fixed", "random"}, "fixed"},
			{"spawnprotection", "出生保护", "behavior", "select", []string{"default", "0", "5", "10", "15", "20", "30"}, "default"},
			{"healthpenalty", "死亡惩罚", "behavior", "toggle", toggleOptions, "true"},
			{"ghostsanitydrain", "幽灵掉san", "behavior", "select", []string{"none", "slow", "medium", "fast"}, "none"},
			{"portalresurection", "绚丽之门复活", "behavior", "toggle", toggleOptions, "true"},
			{"ghostenabled", "幽灵重生", "behavior", "toggle", toggleOptions, "true"},
		},
	},
	{
		Key: "environment", Label: "环境", Icon: "🌿",
		Params: []WorldParam{
			{"weather", "天气强度", "environment", "select", []string{"default", "none", "less", "more"}, "default"},
			{"darkness", "黑暗强度", "environment", "select", []string{"default", "none", "less", "more", "maximum"}, "default"},
			{"temperaturedamage", "温度伤害", "environment", "select", []string{"default", "none", "less", "more"}, "default"},
			{"lightning", "雷暴", "environment", "select", commonSelectOptions, "default"},
			{"wildfires", "野火", "environment", "select", commonSelectOptions, "default"},
			{"meteorshowers", "流星雨", "environment", "select", commonSelectOptions, "default"},
			{"lunarhail_frequency", "月食频率", "environment", "select", commonSelectOptions, "default"},
			{"start_location", "出生点位置", "environment", "select", []string{"default", "forest", "caves"}, "default"},
			{"basicresource_regrowth", "基础资源再生", "environment", "toggle", toggleOptions, "true"},
			{"seasonalstartingitems", "季节初始物品", "environment", "select", []string{"default", "always", "none"}, "default"},
		},
	},
	{
		Key: "resources", Label: "资源", Icon: "🪨",
		Params: []WorldParam{
			{"grass", "草", "resources", "select", commonSelectOptions, "default"},
			{"flint", "燧石", "resources", "select", commonSelectOptions, "default"},
			{"rock", "石头", "resources", "select", commonSelectOptions, "default"},
			{"rock_ice", "冰", "resources", "select", commonSelectOptions, "default"},
			{"sapling", "树苗", "resources", "select", commonSelectOptions, "default"},
			{"reeds", "芦苇", "resources", "select", commonSelectOptions, "default"},
			{"butterfly", "蝴蝶", "resources", "select", commonSelectOptions, "default"},
            {"fishschools", "鱼群", "resources", "select", commonSelectOptions, "default"},
            {"ponds", "池塘", "resources", "select", commonSelectOptions, "default"},
            {"tumbleweed", " tumbleweed", "resources", "select", commonSelectOptions, "default"},
            {"carrot", "胡萝卜", "resources", "select", commonSelectOptions, "default"},
            {"cactus", "仙人掌", "resources", "select", commonSelectOptions, "default"},
            {"berrybush", "浆果丛", "resources", "select", commonSelectOptions, "default"},
            {"mushroom", "蘑菇", "resources", "select", commonSelectOptions, "default"},
            {"flowers", "花", "resources", "select", commonSelectOptions, "default"},
            {"trees", "树", "resources", "select", commonSelectOptions, "default"},
            {"palmconetree", "棕榈树", "resources", "select", commonSelectOptions, "default"},
            {"marshbush", "沼泽丛", "resources", "select", commonSelectOptions, "default"},
		},
	},
	{
		Key: "creatures", Label: "生物", Icon: "🐾",
		Params: []WorldParam{
            {"birds", "鸟", "creatures", "select", commonSelectOptions, "default"},
            {"rabbits", "兔子", "creatures", "select", commonSelectOptions, "default"},
            {"spiders", "蜘蛛", "creatures", "select", commonSelectOptions, "default"},
            {"bees", "蜜蜂", "creatures", "select", commonSelectOptions, "default"},
            {"bats", "蝙蝠", "creatures", "select", commonSelectOptions, "default"},
            {"hounds", "猎犬", "creatures", "select", commonSelectOptions, "default"},
            {"penguins", "企鹅", "creatures", "select", commonSelectOptions, "default"},
            {"pigs", "猪", "creatures", "select", commonSelectOptions, "default"},
            {"merms", "水獭", "creatures", "select", commonSelectOptions, "default"},
            {"moles", "鼹鼠", "creatures", "select", commonSelectOptions, "default"},
            {"wobsters", "龙虾", "creatures", "select", commonSelectOptions, "default"},
            {"walrus", "海象", "creatures", "select", commonSelectOptions, "default"},
            {"krampus", "克朗帕斯", "creatures", "select", commonSelectOptions, "default"},
            {"deerclops", "鹿眼巨人", "creatures", "select", commonSelectOptions, "default"},
            {"dragonfly", "蜻蜓", "creatures", "select", commonSelectOptions, "default"},
            {"beefalo", "野牛", "creatures", "select", commonSelectOptions, "default"},
            {"spiderqueen", "蛛后", "creatures", "select", commonSelectOptions, "default"},
            {"bearger", "熊獾", "creatures", "select", commonSelectOptions, "default"},
            {"goosemoose", "鹅驼鹿", "creatures", "select", commonSelectOptions, "default"},
            {"gnarwail", "歌婆", "creatures", "select", commonSelectOptions, "default"},
            {"lightninggoat", "闪电羊", "creatures", "select", commonSelectOptions, "default"},
            {"malbatross", "巨鲸", "creatures", "select", commonSelectOptions, "default"},
		},
	},
	{
		Key: "season", Label: "季节", Icon: "🍂",
		Params: []WorldParam{
            {"season_start", "初始季节", "season", "select", []string{"default", "spring", "summer", "autumn", "winter", "none"}, "default"},
            {"spring", "春季时长", "season", "select", []string{"default", "none", "very_short", "short", "medium", "long", "very_long", "maximum"}, "default"},
            {"summer", "夏季时长", "season", "select", []string{"default", "none", "very_short", "short", "medium", "long", "very_long", "maximum"}, "default"},
            {"autumn", "秋季时长", "season", "select", []string{"default", "none", "very_short", "short", "medium", "long", "very_long", "maximum"}, "default"},
            {"winter", "冬季时长", "season", "select", []string{"default", "none", "very_short", "short", "medium", "long", "very_long", "maximum"}, "default"},
            {"winters_feast", "冬日盛宴", "season", "select", commonSelectOptions, "default"},
            {"hallowed_nights", "万圣之夜", "season", "select", commonSelectOptions, "default"},
		},
	},
	{
		Key: "ocean", Label: "海洋", Icon: "🌊",
		Params: []WorldParam{
            {"sharks", "鲨鱼", "ocean", "select", commonSelectOptions, "default"},
            {"squid", "鱿鱼", "ocean", "select", commonSelectOptions, "default"},
            {"klaus", "海盗克劳斯", "ocean", "select", commonSelectOptions, "default"},
            {"pirateraids", "海盗袭击", "ocean", "select", commonSelectOptions, "default"},
            {"crabking", "螃蟹王", "ocean", "select", commonSelectOptions, "default"},
            {"tentacles", "触手", "ocean", "select", commonSelectOptions, "default"},
            {"ocean_otterdens", "水獭洞", "ocean", "select", commonSelectOptions, "default"},
            {"ocean_seastack", "海堆", "ocean", "select", commonSelectOptions, "default"},
            {"ocean_shoal", "沙洲", "ocean", "select", commonSelectOptions, "default"},
            {"ocean_bullkelp", "牛藻", "ocean", "select", commonSelectOptions, "default"},
		},
	},
	{
		Key: "moon", Label: "月亮", Icon: "🌙",
		Params: []WorldParam{
            {"moon_berrybush", "月浆果", "moon", "select", commonSelectOptions, "default"},
            {"moon_rock", "月石", "moon", "select", commonSelectOptions, "default"},
            {"moon_carrot", "月萝卜", "moon", "select", commonSelectOptions, "default"},
            {"moon_spider", "月蜘蛛", "moon", "select", commonSelectOptions, "default"},
            {"moon_hotspring", "月温泉", "moon", "select", commonSelectOptions, "default"},
            {"moon_tree", "月树", "moon", "select", commonSelectOptions, "default"},
            {"moon_starfish", "月海星", "moon", "select", commonSelectOptions, "default"},
		},
	},
	{
		Key: "events", Label: "活动", Icon: "🎁",
		Params: []WorldParam{
            {"special_event", "特殊活动", "events", "select", []string{"default", "none", "always"}, "default"},
            {"year_of_the_pig", "猪年", "events", "select", []string{"default", "always", "never"}, "default"},
            {"year_of_the_bunnyman", "兔子年", "events", "select", []string{"default", "always", "never"}, "default"},
            {"year_of_the_catcoon", "猫coon年", "events", "select", []string{"default", "always", "never"}, "default"},
            {"year_of_the_beefalo", "野牛年", "events", "select", []string{"default", "always", "never"}, "default"},
            {"year_of_the_knight", "骑士年", "events", "select", []string{"default", "always", "never"}, "default"},
            {"year_of_the_dragonfly", "蜻蜓年", "events", "select", []string{"default", "always", "never"}, "default"},
            {"year_of_the_varg", "狼年", "events", "select", []string{"default", "always", "never"}, "default"},
        },
    },
}

// 洞穴世界参数分类
var caveParamCategories = []ParamCategory{
    {
        Key: "behavior", Label: "行为", Icon: "🎮",
        Params: []WorldParam{
            {"day", "白天长度", "behavior", "select", []string{"default", "very_short", "short", "medium", "long", "very_long", "maximum"}, "default"},
            {"night", "夜晚长度", "behavior", "select", []string{"default", "very_short", "short", "medium", "long", "very_long", "maximum", "none"}, "default"},
            {"resettime", "重置时间", "behavior", "select", []string{"default", "none", "60", "300", "900", "1800", "3600"}, "default"},
            {"spawnmode", "出生点模式", "behavior", "select", []string{"fixed", "random"}, "fixed"},
            {"spawnprotection", "出生保护", "behavior", "select", []string{"default", "0", "5", "10", "15", "20", "30"}, "default"},
            {"ghostsanitydrain", "幽灵掉san", "behavior", "select", []string{"none", "slow", "medium", "fast"}, "none"},
        },
    },
    {
        Key: "environment", Label: "环境", Icon: "🌿",
        Params: []WorldParam{
            {"weather", "天气强度", "environment", "select", []string{"default", "none", "less", "more"}, "default"},
            {"darkness", "黑暗强度", "environment", "select", []string{"default", "none", "less", "more", "maximum"}, "default"},
            {"temperaturedamage", "温度伤害", "environment", "select", []string{"default", "none", "less", "more"}, "default"},
            {"earthquakes", "地震", "environment", "select", commonSelectOptions, "default"},
            {"cavelight", "洞穴光照", "environment", "select", []string{"default", "none", "less", "more", "always"}, "default"},
            {"wormattacks", "蠕虫攻击", "environment", "select", commonSelectOptions, "default"},
            {"wormattacks_boss", "蠕虫boss", "environment", "select", commonSelectOptions, "default"},
        },
    },
    {
        Key: "resources", Label: "资源", Icon: "🪨",
        Params: []WorldParam{
            {"grass", "草", "resources", "select", commonSelectOptions, "default"},
            {"rock", "石头", "resources", "select", commonSelectOptions, "default"},
            {"flint", "燧石", "resources", "select", commonSelectOptions, "default"},
            {"reeds", "芦苇", "resources", "select", commonSelectOptions, "default"},
            {"mushroom", "蘑菇", "resources", "select", commonSelectOptions, "default"},
            {"lichen", "苔藓", "resources", "select", commonSelectOptions, "default"},
            {"fern", "蕨类", "resources", "select", commonSelectOptions, "default"},
            {"tree_rock", "树石", "resources", "select", commonSelectOptions, "default"},
            {"banana", "香蕉", "resources", "select", commonSelectOptions, "default"},
            {"wormlights", "虫光", "resources", "select", commonSelectOptions, "default"},
        },
    },
    {
        Key: "creatures", Label: "生物", Icon: "🐾",
        Params: []WorldParam{
            {"slurtles", "海龟", "creatures", "select", commonSelectOptions, "default"},
            {"snurtles", "地鼠龟", "creatures", "select", commonSelectOptions, "default"},
            {"cave_spiders", "洞穴蜘蛛", "creatures", "select", commonSelectOptions, "default"},
            {"bunnymen", "兔子人", "creatures", "select", commonSelectOptions, "default"},
            {"spiderqueen", "蛛后", "creatures", "select", commonSelectOptions, "default"},
            {"spider_warriors", "蛛战士", "creatures", "select", commonSelectOptions, "default"},
            {"spiders", "蜘蛛", "creatures", "select", commonSelectOptions, "default"},
            {"lightfliers", "飞虫", "creatures", "select", commonSelectOptions, "default"},
            {"mushgnome", "蘑菇侏儒", "creatures", "select", commonSelectOptions, "default"},
            {"daywalker", "白昼行者", "creatures", "select", commonSelectOptions, "default"},
            {"dustmoths", "尘蛾", "creatures", "select", commonSelectOptions, "default"},
            {"fissure", "裂隙", "creatures", "select", commonSelectOptions, "default"},
        },
    },
}

// GetParamCategories 获取世界参数的分类列表
func GetParamCategories(worldName string) []ParamCategory {
    n := strings.ToLower(worldName)
    if n == "master" || n == "ground" || n == "" {
        return masterParamCategories
    }
    return caveParamCategories
}

// ParseLevelDataToParams 将 leveldataoverride.lua 内容解析为参数 map
func ParseLevelDataToParams(luaContent string) map[string]string {
    params := make(map[string]string)
    lines := strings.Split(luaContent, "\n")
    inOverrides := false

    for _, line := range lines {
        trimmed := strings.TrimSpace(line)

        if strings.HasPrefix(trimmed, "overrides={") {
            inOverrides = true
            continue
        }

        if inOverrides && trimmed == "}," {
            break
        }

        if inOverrides {
            // 格式: key="value",  或 key=true,
            kv := strings.SplitN(trimmed, "=", 2)
            if len(kv) == 2 {
                key := strings.TrimSpace(kv[0])
                val := strings.TrimSpace(kv[1])
                val = strings.TrimSuffix(val, ",")
                val = strings.Trim(val, `"`)
                params[key] = val
            }
        }
    }
    return params
}

// BuildLevelDataFromParams 从参数 map 生成 leveldataoverride.lua
func BuildLevelDataFromParams(worldType string, params map[string]string) string {
    base := GenerateLevelData(worldType, nil)
    for key, value := range params {
        base = replaceParamInLua(base, key, value)
    }
    return base
}

// replaceParamInLua 在 lua 内容中替换指定参数值
func replaceParamInLua(content string, key, value string) string {
    search := key + `="`
    idx := strings.Index(content, search)
    if idx >= 0 {
        start := idx + len(search)
        rest := content[start:]
        quoteIdx := strings.Index(rest, `"`)
        if quoteIdx >= 0 {
            escaped := strings.ReplaceAll(value, `\`, `\\`)
            escaped = strings.ReplaceAll(escaped, `"`, `\"`)
            content = content[:start] + escaped + content[start+quoteIdx:]
        }
        return content
    }
    // 尝试 bool 值替换: key=true 或 key=false
    search2 := key + "="
    idx2 := strings.Index(content, search2)
    if idx2 >= 0 {
        start2 := idx2 + len(search2)
        rest := content[start2:]
        endIdx := -1
        for i, ch := range rest {
            if ch == ',' || ch == '\n' || ch == '}' {
                endIdx = i
                break
            }
        }
        if endIdx >= 0 {
            content = content[:start2] + value + content[start2+endIdx:]
        }
    }
    return content
}
