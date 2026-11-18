package uk.starryheavens.vesper.bothub.navigation

enum class BotHubTab(
    val index: Int,
    val label: String,
    val contentDescription: String
) {
    AGENT(0, "消息", "AI 好友与群聊消息"),
    INBOX(1, "好友", "AI 好友与群聊通讯录"),
    STASH(2, "收藏", "收藏匣与随手闪念"),
    HUB(3, "中枢", "资产守望与个人中枢");

    companion object {
        fun fromIndex(index: Int): BotHubTab? = entries.firstOrNull { it.index == index }
    }
}
