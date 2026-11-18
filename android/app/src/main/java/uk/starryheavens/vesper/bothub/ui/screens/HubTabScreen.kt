package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.background
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.AssetItem
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.ui.components.CrispTagPill
import uk.starryheavens.vesper.bothub.ui.components.MetricMiniCard
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileToggle

@Composable
fun HubTabScreen(
    snapshot: DashboardSnapshot?,
    onRunTask: (TaskItem) -> Unit,
    onToggleTask: (TaskItem, Boolean) -> Unit,
    onSyncAssets: () -> Unit,
    onSaveAsset: (
        existingId: String?,
        name: String,
        category: String,
        spec: String,
        price: String,
        dueDate: String,
        autoRenew: Boolean
    ) -> Unit,
    onDeleteAsset: (String) -> Unit,
    modifier: Modifier = Modifier
) {
    val expiringCount = snapshot?.expiringCount ?: 0
    var showAssetSheet by remember { mutableStateOf(false) }
    var editingAsset by remember { mutableStateOf<AssetItem?>(null) }

    LazyColumn(
        modifier = modifier,
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 10.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        // 1. Summary Tiles
        item(key = "__hub_metrics__") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "资产总数",
                    value = (snapshot?.totalAssets ?: 0).toString(),
                    accentColor = Color(0xFFFFE2BE)
                )
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "7天临期",
                    value = expiringCount.toString(),
                    accentColor = if (expiringCount > 0) Color(0xFFFBBF24) else Color(0xFF34D399)
                )
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "定时任务",
                    value = (snapshot?.activeTasks ?: 0).toString(),
                    accentColor = Color(0xFFA5B4FC)
                )
            }
        }

        // 2. Section 1: Assets (Native Add / Edit / Delete right on tap!)
        item(key = "__hub_assets_hdr__") {
            Spacer(modifier = Modifier.height(2.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "资产",
                    fontSize = 15.5.sp,
                    fontWeight = FontWeight.Bold,
                    color = Color(0xFFFFF8EE),
                    maxLines = 1,
                    softWrap = false
                )

                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    TactileSecondaryButton(
                        text = "添加",
                        leadingIcon = Icons.Default.Add,
                        cornerRadius = 999.dp,
                        minHeight = 33.dp,
                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                        onClick = {
                            editingAsset = null
                            showAssetSheet = true
                        }
                    )
                    TactileSecondaryButton(
                        text = "同步",
                        leadingIcon = Icons.Default.Refresh,
                        accent = true,
                        cornerRadius = 999.dp,
                        minHeight = 33.dp,
                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                        onClick = onSyncAssets
                    )
                }
            }
        }

        val assets = snapshot?.assets ?: emptyList()
        if (assets.isEmpty()) {
            item(key = "__hub_assets_empty__") {
                SpecularGlassCard(
                    cornerRadius = 16.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text(
                        text = "暂无记录",
                        fontSize = 13.sp,
                        color = Color(0xFF94A3B8),
                        modifier = Modifier.padding(16.dp)
                    )
                }
            }
        }

        items(assets, key = { it.id }) { asset ->
            val isPermanent = asset.dueDate.startsWith("2099") || asset.daysLeft > 3650
            val isMultiYear = !isPermanent && asset.daysLeft > 365
            val barColor = when {
                isPermanent || isMultiYear -> Color(0xFF34D399)
                asset.daysLeft <= 7 -> Color(0xFFFB7185)
                asset.daysLeft <= 30 -> Color(0xFFFBBF24)
                else -> Color(0xFFF2B880)
            }
            val progressFraction = when {
                isPermanent -> 1f
                asset.daysLeft <= 0 -> 0.04f
                else -> (asset.daysLeft.toFloat() / 365f).coerceIn(0.06f, 1f)
            }

            SpecularGlassCard(
                cornerRadius = 18.dp,
                accentTint = if (asset.isExpiringSoon && !isPermanent) barColor else null,
                onClick = {
                    editingAsset = asset
                    showAssetSheet = true
                },
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(horizontal = 15.dp, vertical = 13.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = asset.name,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE),
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        CrispTagPill(
                            text = if (isPermanent) "长期" else asset.dueBadgeLabel,
                            bgColor = barColor.copy(alpha = 0.14f),
                            borderColor = barColor.copy(alpha = 0.42f),
                            textColor = barColor
                        )
                    }

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = listOf(asset.spec, asset.price)
                                .filter { it.isNotBlank() }
                                .joinToString(" · ")
                                .ifEmpty { "—" },
                            fontSize = 12.5.sp,
                            color = Color(0xFF8C9BB5),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 12.dp)
                        )
                        if (!isPermanent && asset.dueDate.isNotEmpty()) {
                            Text(
                                text = asset.dueDate.take(10),
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                color = Color(0xFF5D6D89),
                                maxLines = 1,
                                softWrap = false
                            )
                        }
                    }

                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(3.5.dp)
                            .clip(RoundedCornerShape(999.dp))
                            .background(Color(0xFF141C34))
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(progressFraction)
                                .height(3.5.dp)
                                .clip(RoundedCornerShape(999.dp))
                                .background(
                                    Brush.horizontalGradient(
                                        listOf(barColor.copy(alpha = 0.72f), barColor)
                                    )
                                )
                        )
                    }
                }
            }
        }

        // 3. Section 2: Scheduled Tasks
        item(key = "__hub_tasks_hdr__") {
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = "定时任务",
                fontSize = 15.5.sp,
                fontWeight = FontWeight.Bold,
                color = Color(0xFFFFF8EE),
                maxLines = 1,
                softWrap = false
            )
        }

        val tasks = snapshot?.tasks ?: emptyList()
        items(tasks, key = { it.id }) { task ->
            SpecularGlassCard(
                cornerRadius = 18.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(horizontal = 15.dp, vertical = 12.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = task.name,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = if (task.enabled) Color(0xFFFFF8EE) else Color(0xFF8C9BB5),
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        TactileToggle(
                            checked = task.enabled,
                            onCheckedChange = { checked -> onToggleTask(task, checked) }
                        )
                    }

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = task.formattedSchedule(),
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.5.sp,
                            color = Color(0xFF8C9BB5),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )

                        TactileSecondaryButton(
                            text = "运行",
                            leadingIcon = Icons.Default.PlayArrow,
                            accent = true,
                            cornerRadius = 999.dp,
                            minHeight = 33.dp,
                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                            onClick = { onRunTask(task) }
                        )
                    }
                }
            }
        }
    }

    if (showAssetSheet) {
        AssetEditorSheet(
            asset = editingAsset,
            onSave = { name, category, spec, price, dueDate, autoRenew ->
                onSaveAsset(editingAsset?.id, name, category, spec, price, dueDate, autoRenew)
                showAssetSheet = false
            },
            onDelete = editingAsset?.let { target ->
                {
                    onDeleteAsset(target.id)
                    showAssetSheet = false
                }
            },
            onDismiss = { showAssetSheet = false }
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AssetEditorSheet(
    asset: AssetItem?,
    onSave: (
        name: String,
        category: String,
        spec: String,
        price: String,
        dueDate: String,
        autoRenew: Boolean
    ) -> Unit,
    onDelete: (() -> Unit)?,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var name by remember(asset) { mutableStateOf(asset?.name.orEmpty()) }
    var category by remember(asset) { mutableStateOf(asset?.category?.ifBlank { "compute" } ?: "compute") }
    var spec by remember(asset) { mutableStateOf(asset?.spec.orEmpty()) }
    var price by remember(asset) { mutableStateOf(asset?.price.orEmpty()) }
    var dueDate by remember(asset) { mutableStateOf(asset?.dueDate?.take(10).orEmpty()) }
    var autoRenew by remember(asset) { mutableStateOf(asset?.autoRenew ?: false) }

    val categories = listOf(
        "compute" to "服务器",
        "domain" to "域名",
        "subscription" to "订阅会员"
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
                text = if (asset == null) "添加资产" else "编辑资产",
                fontSize = 17.5.sp,
                fontWeight = FontWeight.Bold,
                color = Color(0xFFFFF8EE)
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                categories.forEach { (cid, label) ->
                    TactileSecondaryButton(
                        text = label,
                        accent = category == cid,
                        cornerRadius = 11.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 6.dp),
                        onClick = { category = cid },
                        modifier = Modifier.weight(1f)
                    )
                }
            }

            ObsidianGlassInput(
                value = name,
                onValueChange = { name = it },
                label = "名称",
                placeholder = "例如：CloudCone / 主域名 / 会员",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            ObsidianGlassInput(
                value = spec,
                onValueChange = { spec = it },
                label = "说明 (可选)",
                placeholder = "配置或备注",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                ObsidianGlassInput(
                    value = price,
                    onValueChange = { price = it },
                    label = "费用 (可选)",
                    placeholder = "例如：¥85/年",
                    singleLine = true,
                    modifier = Modifier.weight(1f)
                )
                ObsidianGlassInput(
                    value = dueDate,
                    onValueChange = { dueDate = it },
                    label = "到期日",
                    placeholder = "YYYY-MM-DD",
                    singleLine = true,
                    modifier = Modifier.weight(1f)
                )
            }

            SpecularGlassCard(
                cornerRadius = 15.dp,
                onClick = { autoRenew = !autoRenew },
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { autoRenew = !autoRenew }
                        .padding(horizontal = 14.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("自动续费", fontSize = 14.sp, color = Color(0xFFFFF8EE))
                    TactileToggle(
                        checked = autoRenew,
                        onCheckedChange = { autoRenew = it }
                    )
                }
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                if (onDelete != null) {
                    TactileSecondaryButton(
                        text = "删除",
                        danger = true,
                        minHeight = 46.dp,
                        onClick = onDelete,
                        modifier = Modifier.weight(1f)
                    )
                }
                TactilePrimaryButton(
                    text = "保存",
                    enabled = name.isNotBlank(),
                    minHeight = 46.dp,
                    onClick = {
                        if (name.isNotBlank()) {
                            onSave(
                                name.trim(),
                                category,
                                spec.trim(),
                                price.trim(),
                                dueDate.trim().ifEmpty { "2099-12-31" },
                                autoRenew
                            )
                        }
                    },
                    modifier = Modifier.weight(if (onDelete != null) 1.5f else 1f)
                )
            }

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}
