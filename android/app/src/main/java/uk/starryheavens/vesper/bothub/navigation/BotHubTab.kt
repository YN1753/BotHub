package uk.starryheavens.vesper.bothub.navigation

enum class BotHubTab(
    val index: Int,
    val label: String,
    val contentDescription: String
) {
    AGENT(0, "对话", "智能中枢对话"),
    INBOX(1, "情报箱", "原生情报收件箱"),
    STASH(2, "收藏匣", "智能收藏与闪念匣"),
    HUB(3, "中枢", "资产与调度中枢");

    companion object {
        fun fromIndex(index: Int): BotHubTab? = entries.firstOrNull { it.index == index }
    }
}
