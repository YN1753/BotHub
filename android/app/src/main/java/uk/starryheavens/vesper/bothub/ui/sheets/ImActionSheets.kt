package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ChatBubble
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.CrispTagPill
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.SpiritSquircleAvatar
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileToggle
import uk.starryheavens.vesper.bothub.ui.components.specularRimBrush
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable

private val AVATAR_OPTIONS = listOf(
    "butler:gold" to "暖金",
    "sentinel:indigo" to "星靛",
    "radar:emerald" to "翡翠",
    "swarm:violet" to "紫晶",
    "scholar:cyan" to "青岚",
    "critic:rose" to "绯红"
)

/**
 * 1. Contact / Group Profile Card Sheet ("资料卡").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BotProfileSheet(
    bot: BotItem,
    allBots: List<BotItem>,
    onSendMessage: () -> Unit,
    onTogglePin: () -> Unit,
    onStartGroupWithBot: () -> Unit,
    onEditBot: () -> Unit,
    onDeleteBot: () -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val members = remember(bot, allBots) {
        if (bot.isGroup) bot.resolveGroupMembers(allBots) else emptyList()
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = Color(0xFF080D1B),
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .padding(horizontal = 20.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(13.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(14.dp)
            ) {
                SpiritSquircleAvatar(
                    bot = bot,
                    allBots = allBots,
                    containerSize = 62.dp,
                    cornerRadius = 18.dp,
                    showPresenceDot = true,
                    interactive = true
                )

                Column(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(5.dp)
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Text(
                            text = bot.name,
                            fontSize = 19.sp,
                            fontWeight = FontWeight.Bold,
                            color = Color(0xFFFFF8EE),
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false)
                        )
                        CrispTagPill(
                            text = if (bot.isGroup) bot.groupModeLabel() else bot.modeBadgeLabel(),
                            bgColor = Color(0x28F2B880),
                            borderColor = Color(0x5CF2B880),
                            textColor = Color(0xFFFFE2BE)
                        )
                    }

                    val subtitle = bot.roleTitle.ifBlank { if (bot.isGroup) "群聊" else "AI 好友" }
                    Text(
                        text = subtitle,
                        fontSize = 12.5.sp,
                        color = Color(0xFF8C9BB5),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }

            if (bot.traits.isNotBlank() || (bot.isGroup && members.isNotEmpty())) {
                SpecularGlassCard(
                    cornerRadius = 18.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 15.dp, vertical = 13.dp),
                        verticalArrangement = Arrangement.spacedBy(9.dp)
                    ) {
                        if (bot.traits.isNotBlank()) {
                            ProfileFieldRow(
                                label = "简介",
                                value = bot.traits
                            )
                        }
                        if (bot.traits.isNotBlank() && bot.isGroup && members.isNotEmpty()) {
                            HorizontalDivider(thickness = 0.5.dp, color = Color(0x14FFF8EE))
                        }
                        if (bot.isGroup && members.isNotEmpty()) {
                            Column(verticalArrangement = Arrangement.spacedBy(7.dp)) {
                                Text(
                                    text = "成员 (${members.size})",
                                    fontSize = 11.5.sp,
                                    color = Color(0xFF8C9BB5),
                                    maxLines = 1,
                                    softWrap = false
                                )
                                LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                    items(members, key = { it.id }) { m ->
                                        Column(
                                            horizontalAlignment = Alignment.CenterHorizontally,
                                            verticalArrangement = Arrangement.spacedBy(4.dp)
                                        ) {
                                            SpiritSquircleAvatar(
                                                bot = m,
                                                allBots = allBots,
                                                containerSize = 40.dp,
                                                cornerRadius = 12.dp
                                            )
                                            Text(
                                                text = m.name,
                                                fontSize = 10.5.sp,
                                                color = Color(0xFFCBD5E1),
                                                textAlign = TextAlign.Center,
                                                maxLines = 1,
                                                softWrap = false,
                                                overflow = TextOverflow.Ellipsis,
                                                modifier = Modifier.width(50.dp)
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            SpecularGlassCard(
                cornerRadius = 16.dp,
                onClick = onTogglePin,
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 15.dp, vertical = 11.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "置顶聊天",
                        fontSize = 14.sp,
                        fontWeight = FontWeight.Medium,
                        color = Color(0xFFFFF8EE),
                        maxLines = 1,
                        softWrap = false
                    )
                    TactileToggle(
                        checked = bot.isPinned,
                        onCheckedChange = { onTogglePin() }
                    )
                }
            }

            TactilePrimaryButton(
                text = if (bot.isGroup) "进入群聊" else "发消息",
                leadingIcon = Icons.Default.ChatBubble,
                minHeight = 48.dp,
                onClick = onSendMessage,
                modifier = Modifier.fillMaxWidth()
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                if (!bot.isGroup) {
                    TactileSecondaryButton(
                        text = "发起群聊",
                        minHeight = 42.dp,
                        onClick = onStartGroupWithBot,
                        modifier = Modifier.weight(1f)
                    )
                }
                TactileSecondaryButton(
                    text = "编辑资料",
                    minHeight = 42.dp,
                    onClick = onEditBot,
                    modifier = Modifier.weight(1f)
                )
                if (!bot.isBuiltin) {
                    TactileSecondaryButton(
                        text = if (bot.isGroup) "解散群聊" else "删除好友",
                        danger = true,
                        minHeight = 42.dp,
                        onClick = onDeleteBot,
                        modifier = Modifier.weight(1f)
                    )
                }
            }

            Spacer(modifier = Modifier.height(6.dp))
        }
    }
}

@Composable
private fun ProfileFieldRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = label,
            fontSize = 12.sp,
            color = Color(0xFF8C9BB5),
            maxLines = 1,
            softWrap = false,
            modifier = Modifier.width(42.dp)
        )
        Text(
            text = value,
            fontSize = 13.sp,
            lineHeight = 19.sp,
            color = Color(0xFFE6ECF8),
            modifier = Modifier.weight(1f)
        )
    }
}

/**
 * 2. Native Edit Friend / Group Profile Sheet (Zero WebView jump!).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EditBotSheet(
    bot: BotItem,
    onSave: (
        name: String,
        roleTitle: String,
        avatarStyle: String,
        traits: String,
        systemPrompt: String,
        groupMode: String,
        announcement: String
    ) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var name by remember(bot.id) { mutableStateOf(bot.name) }
    var roleTitle by remember(bot.id) { mutableStateOf(bot.roleTitle) }
    var avatarStyle by remember(bot.id) { mutableStateOf(bot.avatarStyle.ifBlank { "butler:gold" }) }
    var traits by remember(bot.id) { mutableStateOf(bot.traits) }
    var systemPrompt by remember(bot.id) { mutableStateOf(bot.systemPrompt) }
    var groupMode by remember(bot.id) { mutableStateOf(bot.groupMode.ifBlank { "smart" }) }
    var announcement by remember(bot.id) { mutableStateOf(bot.announcement) }

    val groupModes = listOf(
        "smart" to "智能协同",
        "sequential" to "轮流接力",
        "parallel" to "全员并发"
    )

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = Color(0xFF080D1B),
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = if (bot.isGroup) "编辑群资料" else "编辑好友资料",
                fontSize = 17.5.sp,
                fontWeight = FontWeight.Bold,
                color = Color(0xFFFFF8EE)
            )

            if (!bot.isGroup) {
                AvatarPickerRow(
                    selected = avatarStyle,
                    onSelect = { avatarStyle = it }
                )
            }

            ObsidianGlassInput(
                value = name,
                onValueChange = { name = it },
                label = if (bot.isGroup) "群名称" else "昵称",
                placeholder = "输入名称",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            ObsidianGlassInput(
                value = roleTitle,
                onValueChange = { roleTitle = it },
                label = if (bot.isGroup) "群定位" else "头衔",
                placeholder = "简短描述角色",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            if (bot.isGroup) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    groupModes.forEach { (mid, mLabel) ->
                        TactileSecondaryButton(
                            text = mLabel,
                            accent = groupMode == mid,
                            cornerRadius = 11.dp,
                            minHeight = 38.dp,
                            contentPadding = PaddingValues(horizontal = 8.dp, vertical = 7.dp),
                            onClick = { groupMode = mid },
                            modifier = Modifier.weight(1f)
                        )
                    }
                }

                ObsidianGlassInput(
                    value = announcement,
                    onValueChange = { announcement = it },
                    label = "群公告 (可选)",
                    placeholder = "输入群公告...",
                    singleLine = false,
                    maxLines = 3,
                    modifier = Modifier.fillMaxWidth()
                )
            } else {
                ObsidianGlassInput(
                    value = traits,
                    onValueChange = { traits = it },
                    label = "简介与说话风格 (可选)",
                    placeholder = "例如：言简意赅、自然直率",
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )

                ObsidianGlassInput(
                    value = systemPrompt,
                    onValueChange = { systemPrompt = it },
                    label = "系统提示词",
                    placeholder = "设定角色指令与边界...",
                    singleLine = false,
                    maxLines = 4,
                    modifier = Modifier.fillMaxWidth()
                )
            }

            TactilePrimaryButton(
                text = "保存",
                enabled = name.isNotBlank(),
                minHeight = 48.dp,
                onClick = {
                    if (name.isNotBlank()) {
                        onSave(
                            name.trim(),
                            roleTitle.trim(),
                            avatarStyle,
                            traits.trim(),
                            systemPrompt.trim(),
                            groupMode,
                            announcement.trim()
                        )
                    }
                },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}

@Composable
private fun AvatarPickerRow(
    selected: String,
    onSelect: (String) -> Unit
) {
    LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        items(AVATAR_OPTIONS) { (styleId, label) ->
            val active = selected == styleId
            val itemShape = RoundedCornerShape(15.dp)
            Column(
                modifier = Modifier
                    .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                        onSelect(styleId)
                    }
                    .clip(itemShape)
                    .background(
                        Brush.verticalGradient(
                            if (active) {
                                listOf(Color(0x33F2B880), Color(0x1AC88A4E))
                            } else {
                                listOf(Color(0xFF141D37), Color(0xFF0B1020))
                            }
                        ),
                        itemShape
                    )
                    .border(
                        BorderStroke(
                            0.85.dp,
                            if (active) {
                                specularRimBrush(Color(0x88F2B880), Color(0x40F2B880), Color.Transparent)
                            } else {
                                specularRimBrush(Color(0x28FFF8EE), Color(0x10FFF8EE), Color.Transparent)
                            }
                        ),
                        itemShape
                    )
                    .padding(horizontal = 12.dp, vertical = 9.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                GrokBotAvatar(
                    avatarStyle = styleId,
                    state = if (active) "working" else "idle",
                    size = 36.dp,
                    interactive = false,
                    seedKey = styleId
                )
                Text(
                    text = label,
                    fontSize = 10.5.sp,
                    color = if (active) Color(0xFFFFE2BE) else Color(0xFF94A3B8),
                    maxLines = 1,
                    softWrap = false
                )
            }
        }
    }
}

/**
 * 3. Native Quick-Add AI Friend Sheet ("添加 AI 好友").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CreateFriendSheet(
    onSubmit: (
        name: String,
        roleTitle: String,
        avatarStyle: String,
        botMode: String,
        traits: String,
        systemPrompt: String
    ) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val modeOptions = listOf(
        "companion" to "日常伴聊",
        "hybrid" to "全能伙伴",
        "worker" to "工具助理"
    )

    var selectedAvatar by remember { mutableStateOf("swarm:violet") }
    var selectedMode by remember { mutableStateOf("hybrid") }
    var name by remember { mutableStateOf("") }
    var roleTitle by remember { mutableStateOf("") }
    var traits by remember { mutableStateOf("") }
    var systemPrompt by remember { mutableStateOf("") }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = Color(0xFF080D1B),
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = "添加 AI 好友",
                fontSize = 17.5.sp,
                fontWeight = FontWeight.Bold,
                color = Color(0xFFFFF8EE)
            )

            AvatarPickerRow(
                selected = selectedAvatar,
                onSelect = { selectedAvatar = it }
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                modeOptions.forEach { (mid, mLabel) ->
                    TactileSecondaryButton(
                        text = mLabel,
                        accent = selectedMode == mid,
                        cornerRadius = 11.dp,
                        minHeight = 38.dp,
                        contentPadding = PaddingValues(horizontal = 6.dp, vertical = 7.dp),
                        onClick = { selectedMode = mid },
                        modifier = Modifier.weight(1f)
                    )
                }
            }

            ObsidianGlassInput(
                value = name,
                onValueChange = { name = it },
                label = "昵称",
                placeholder = "例如：架构顾问 / 英语陪练",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            ObsidianGlassInput(
                value = roleTitle,
                onValueChange = { roleTitle = it },
                label = "头衔 (可选)",
                placeholder = "一句话角色描述",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            ObsidianGlassInput(
                value = traits,
                onValueChange = { traits = it },
                label = "说话风格 (可选)",
                placeholder = "例如：言简意赅、自然直率",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            ObsidianGlassInput(
                value = systemPrompt,
                onValueChange = { systemPrompt = it },
                label = "系统提示词 (可选)",
                placeholder = "补充设定或习惯...",
                singleLine = false,
                maxLines = 3,
                modifier = Modifier.fillMaxWidth()
            )

            TactilePrimaryButton(
                text = "完成",
                enabled = name.isNotBlank(),
                minHeight = 48.dp,
                onClick = {
                    val cleanName = name.trim()
                    if (cleanName.isNotEmpty()) {
                        val prompt = systemPrompt.trim().ifEmpty {
                            "你是用户的专属 AI 好友「$cleanName」（${roleTitle.ifBlank { "智能伙伴" }}）。${traits.ifBlank { "交流自然真诚、条理清晰" }}。严禁使用任何 Emoji。"
                        }
                        onSubmit(
                            cleanName,
                            roleTitle.trim().ifEmpty { "AI 好友" },
                            selectedAvatar,
                            selectedMode,
                            traits.trim(),
                            prompt
                        )
                    }
                },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}

/**
 * 4. Select Friends to Start Group Chat Sheet ("发起群聊").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CreateGroupSheet(
    allBots: List<BotItem>,
    sourceBotId: String,
    onSubmit: (
        sourceBotId: String,
        memberIds: List<String>,
        name: String,
        groupMode: String,
        carryContext: Boolean
    ) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val candidates = remember(allBots) { allBots.filter { !it.isGroup } }
    val selectedIds = remember(sourceBotId) {
        mutableStateListOf<String>().apply {
            if (sourceBotId.isNotBlank()) add(sourceBotId)
        }
    }
    var groupName by remember { mutableStateOf("") }
    var groupMode by remember { mutableStateOf("smart") }
    var carryContext by remember(sourceBotId) { mutableStateOf(sourceBotId.isNotBlank()) }

    val modes = listOf(
        "smart" to "智能点名",
        "sequential" to "轮流接力",
        "parallel" to "全员并发"
    )

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = Color(0xFF080D1B),
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "发起群聊",
                    fontSize = 17.5.sp,
                    fontWeight = FontWeight.Bold,
                    color = Color(0xFFFFF8EE),
                    maxLines = 1,
                    softWrap = false
                )

                TactilePrimaryButton(
                    text = "完成 (${selectedIds.size})",
                    enabled = selectedIds.size >= 2,
                    cornerRadius = 999.dp,
                    minHeight = 36.dp,
                    contentPadding = PaddingValues(horizontal = 15.dp, vertical = 6.dp),
                    onClick = {
                        if (selectedIds.size >= 2) {
                            onSubmit(sourceBotId, selectedIds.toList(), groupName, groupMode, carryContext)
                        }
                    }
                )
            }

            ObsidianGlassInput(
                value = groupName,
                onValueChange = { groupName = it },
                placeholder = "群名称 (可选)",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                modes.forEach { (mid, mLabel) ->
                    TactileSecondaryButton(
                        text = mLabel,
                        accent = groupMode == mid,
                        cornerRadius = 11.dp,
                        minHeight = 38.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 7.dp),
                        onClick = { groupMode = mid },
                        modifier = Modifier.weight(1f)
                    )
                }
            }

            if (sourceBotId.isNotBlank()) {
                SpecularGlassCard(
                    cornerRadius = 15.dp,
                    onClick = { carryContext = !carryContext },
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 14.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "同步最近单聊记录",
                            fontSize = 13.5.sp,
                            color = Color(0xFFE6ECF8),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        TactileToggle(
                            checked = carryContext,
                            onCheckedChange = { carryContext = it }
                        )
                    }
                }
            }

            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f, fill = false),
                verticalArrangement = Arrangement.spacedBy(8.dp),
                contentPadding = PaddingValues(bottom = 16.dp)
            ) {
                items(candidates, key = { it.id }) { friend ->
                    val checked = selectedIds.contains(friend.id)
                    val locked = friend.id == sourceBotId
                    SpecularGlassCard(
                        cornerRadius = 15.dp,
                        accentTint = if (checked) Color(0xFFF2B880) else null,
                        onClick = {
                            if (!locked) {
                                if (checked) selectedIds.remove(friend.id) else selectedIds.add(friend.id)
                            }
                        },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(22.dp)
                                    .clip(CircleShape)
                                    .background(
                                        if (checked) {
                                            Brush.verticalGradient(listOf(Color(0xFFF6C692), Color(0xFFDF9855)))
                                        } else {
                                            Brush.verticalGradient(listOf(Color(0xFF141C34), Color(0xFF0C1122)))
                                        },
                                        CircleShape
                                    )
                                    .border(
                                        0.8.dp,
                                        if (checked) Color(0x80FFE8CC) else Color(0x28FFF8EE),
                                        CircleShape
                                    ),
                                contentAlignment = Alignment.Center
                            ) {
                                if (checked) {
                                    Icon(
                                        imageVector = Icons.Default.Check,
                                        contentDescription = null,
                                        tint = Color(0xFF090C16),
                                        modifier = Modifier.size(14.dp)
                                    )
                                }
                            }

                            SpiritSquircleAvatar(
                                bot = friend,
                                allBots = allBots,
                                containerSize = 40.dp,
                                cornerRadius = 12.dp
                            )

                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    text = friend.name,
                                    fontSize = 14.5.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color(0xFFFFF8EE),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                                if (friend.roleTitle.isNotBlank()) {
                                    Text(
                                        text = friend.roleTitle,
                                        fontSize = 11.5.sp,
                                        color = Color(0xFF8C9BB5),
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

/**
 * 5. WeChat "聊天详情 (···)" Drawer inside ChatRoomScreen.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChatInfoSheet(
    activeBot: BotItem?,
    allBots: List<BotItem>,
    activeModel: String,
    enableThinking: Boolean,
    onModelChange: (String) -> Unit,
    onThinkingChange: (Boolean) -> Unit,
    onTogglePin: () -> Unit,
    onInviteToGroup: () -> Unit,
    onOpenMemberProfile: (BotItem) -> Unit,
    onSearchInChat: () -> Unit = {},
    onEditBot: () -> Unit,
    onClearHistory: () -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val isGroup = activeBot?.isGroup == true
    val members = remember(activeBot, allBots) {
        when {
            activeBot == null -> emptyList()
            activeBot.isGroup -> activeBot.resolveGroupMembers(allBots)
            else -> listOf(activeBot)
        }
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = Color(0xFF080D1B),
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = if (isGroup) "群聊详情 (${members.size})" else "聊天详情",
                fontSize = 17.sp,
                fontWeight = FontWeight.Bold,
                color = Color(0xFFFFF8EE)
            )

            SpecularGlassCard(
                cornerRadius = 18.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 14.dp, vertical = 14.dp),
                    horizontalArrangement = Arrangement.spacedBy(14.dp)
                ) {
                    items(members, key = { it.id }) { m ->
                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(5.dp),
                            modifier = Modifier.tactileClickable(pressedScale = 0.93f) {
                                onDismiss()
                                onOpenMemberProfile(m)
                            }
                        ) {
                            SpiritSquircleAvatar(
                                bot = m,
                                allBots = allBots,
                                containerSize = 48.dp,
                                cornerRadius = 14.dp
                            )
                            Text(
                                text = m.name,
                                fontSize = 11.sp,
                                color = Color(0xFFCBD5E1),
                                textAlign = TextAlign.Center,
                                maxLines = 1,
                                softWrap = false,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.width(52.dp)
                            )
                        }
                    }

                    item(key = "__invite_plus__") {
                        val plusShape = RoundedCornerShape(14.dp)
                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(5.dp),
                            modifier = Modifier.tactileClickable(pressedScale = 0.93f) {
                                onDismiss()
                                onInviteToGroup()
                            }
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(48.dp)
                                    .clip(plusShape)
                                    .background(
                                        Brush.verticalGradient(listOf(Color(0xFF151E39), Color(0xFF0C1224))),
                                        plusShape
                                    )
                                    .border(
                                        BorderStroke(
                                            0.85.dp,
                                            specularRimBrush(Color(0x38FFF8EE), Color(0x16FFF8EE), Color.Transparent)
                                        ),
                                        plusShape
                                    ),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Default.Add,
                                    contentDescription = "发起群聊",
                                    tint = Color(0xFF94A3B8),
                                    modifier = Modifier.size(22.dp)
                                )
                            }
                            Text(
                                text = if (isGroup) "管理" else "添加",
                                fontSize = 11.sp,
                                color = Color(0xFF94A3B8),
                                maxLines = 1,
                                softWrap = false
                            )
                        }
                    }
                }
            }

            SpecularGlassCard(
                cornerRadius = 18.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(modifier = Modifier.fillMaxWidth()) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable {
                                onDismiss()
                                onSearchInChat()
                            }
                            .padding(horizontal = 15.dp, vertical = 13.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text("查找聊天内容", fontSize = 14.sp, color = Color(0xFFFFF8EE))
                        Icon(
                            imageVector = Icons.Default.KeyboardArrowRight,
                            contentDescription = null,
                            tint = Color(0xFFF2B880),
                            modifier = Modifier.size(16.dp)
                        )
                    }
                    HorizontalDivider(
                        thickness = 0.5.dp,
                        color = Color(0x14FFF8EE),
                        modifier = Modifier.padding(horizontal = 15.dp)
                    )

                    if (activeBot != null) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable { onTogglePin() }
                                .padding(horizontal = 15.dp, vertical = 12.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text("置顶聊天", fontSize = 14.sp, color = Color(0xFFFFF8EE))
                            TactileToggle(
                                checked = activeBot.isPinned,
                                onCheckedChange = { onTogglePin() }
                            )
                        }
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x14FFF8EE),
                            modifier = Modifier.padding(horizontal = 15.dp)
                        )
                    }

                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable { onThinkingChange(!enableThinking) }
                            .padding(horizontal = 15.dp, vertical = 12.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text("深度思考", fontSize = 14.sp, color = Color(0xFFFFF8EE))
                        TactileToggle(
                            checked = enableThinking,
                            onCheckedChange = onThinkingChange
                        )
                    }

                    HorizontalDivider(
                        thickness = 0.5.dp,
                        color = Color(0x14FFF8EE),
                        modifier = Modifier.padding(horizontal = 15.dp)
                    )

                    val isPool = activeModel == "free-pool"
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable {
                                onModelChange(if (isPool) "deepseek-flash" else "free-pool")
                            }
                            .padding(horizontal = 15.dp, vertical = 13.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text("模型切换", fontSize = 14.sp, color = Color(0xFFFFF8EE))
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(2.dp)
                        ) {
                            Text(
                                text = if (isPool) "免费模型" else "主力模型",
                                fontSize = 12.5.sp,
                                color = Color(0xFFFFE2BE),
                                maxLines = 1,
                                softWrap = false
                            )
                            Icon(
                                imageVector = Icons.Default.KeyboardArrowRight,
                                contentDescription = null,
                                tint = Color(0xFFF2B880),
                                modifier = Modifier.size(16.dp)
                            )
                        }
                    }

                    if (activeBot != null) {
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x14FFF8EE),
                            modifier = Modifier.padding(horizontal = 15.dp)
                        )

                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable {
                                    onDismiss()
                                    onEditBot()
                                }
                                .padding(horizontal = 15.dp, vertical = 13.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = if (isGroup) "编辑群资料" else "编辑好友资料",
                                fontSize = 14.sp,
                                color = Color(0xFFFFF8EE)
                            )
                            Icon(
                                imageVector = Icons.Default.KeyboardArrowRight,
                                contentDescription = null,
                                tint = Color(0xFF5D6D89),
                                modifier = Modifier.size(16.dp)
                            )
                        }
                    }
                }
            }

            TactileSecondaryButton(
                text = "清空聊天记录",
                danger = true,
                minHeight = 46.dp,
                onClick = {
                    onDismiss()
                    onClearHistory()
                },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}

/**
 * 6. Manage Existing Group Members Sheet ("管理群成员").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ManageGroupMembersSheet(
    groupBot: BotItem,
    allBots: List<BotItem>,
    onSubmit: (memberIds: List<String>) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val candidates = remember(allBots) { allBots.filter { !it.isGroup } }
    val selectedIds = remember(groupBot.id, groupBot.memberIds) {
        mutableStateListOf<String>().apply {
            addAll(groupBot.memberIds)
        }
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = Color(0xFF080D1B),
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "群成员管理",
                    fontSize = 17.5.sp,
                    fontWeight = FontWeight.Bold,
                    color = Color(0xFFFFF8EE),
                    maxLines = 1,
                    softWrap = false
                )

                TactilePrimaryButton(
                    text = "保存 (${selectedIds.size})",
                    enabled = selectedIds.isNotEmpty(),
                    cornerRadius = 999.dp,
                    minHeight = 36.dp,
                    contentPadding = PaddingValues(horizontal = 15.dp, vertical = 6.dp),
                    onClick = {
                        if (selectedIds.isNotEmpty()) {
                            onSubmit(selectedIds.toList())
                        }
                    }
                )
            }

            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f, fill = false),
                verticalArrangement = Arrangement.spacedBy(8.dp),
                contentPadding = PaddingValues(bottom = 16.dp)
            ) {
                items(candidates, key = { "mgr_${it.id}" }) { friend ->
                    val checked = selectedIds.contains(friend.id)
                    SpecularGlassCard(
                        cornerRadius = 15.dp,
                        accentTint = if (checked) Color(0xFFF2B880) else null,
                        onClick = {
                            if (checked) {
                                if (selectedIds.size > 1) selectedIds.remove(friend.id)
                            } else {
                                selectedIds.add(friend.id)
                            }
                        },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(22.dp)
                                    .clip(CircleShape)
                                    .background(
                                        if (checked) {
                                            Brush.verticalGradient(listOf(Color(0xFFF6C692), Color(0xFFDF9855)))
                                        } else {
                                            Brush.verticalGradient(listOf(Color(0xFF141C34), Color(0xFF0C1122)))
                                        },
                                        CircleShape
                                    )
                                    .border(
                                        0.8.dp,
                                        if (checked) Color(0x80FFE8CC) else Color(0x28FFF8EE),
                                        CircleShape
                                    ),
                                contentAlignment = Alignment.Center
                            ) {
                                if (checked) {
                                    Icon(
                                        imageVector = Icons.Default.Check,
                                        contentDescription = null,
                                        tint = Color(0xFF090C16),
                                        modifier = Modifier.size(14.dp)
                                    )
                                }
                            }

                            SpiritSquircleAvatar(
                                bot = friend,
                                allBots = allBots,
                                containerSize = 40.dp,
                                cornerRadius = 12.dp
                            )

                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    text = friend.name,
                                    fontSize = 14.5.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color(0xFFFFF8EE),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                                if (friend.roleTitle.isNotBlank()) {
                                    Text(
                                        text = friend.roleTitle,
                                        fontSize = 11.5.sp,
                                        color = Color(0xFF8C9BB5),
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
