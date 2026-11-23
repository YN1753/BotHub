package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountBalanceWallet
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.CalendarMonth
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Inventory2
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material.icons.filled.LocalOffer
import androidx.compose.material.icons.filled.Public
import androidx.compose.material.icons.filled.Security
import androidx.compose.material.icons.filled.VpnKey
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.time.LocalDate
import java.time.YearMonth
import java.util.Locale

data class CalendarDayCell(
    val date: LocalDate,
    val inMonth: Boolean,
    val isToday: Boolean
)

fun assetCategoryIconVector(iconId: String): ImageVector = when (iconId.trim().lowercase()) {
    "server" -> Icons.Default.Dns
    "globe" -> Icons.Default.Public
    "sparkles" -> Icons.Default.AutoAwesome
    "key" -> Icons.Default.VpnKey
    "shield" -> Icons.Default.Security
    "package" -> Icons.Default.Inventory2
    "wallet" -> Icons.Default.AccountBalanceWallet
    else -> Icons.Default.LocalOffer
}

fun parseLocalDateOrNull(raw: String): LocalDate? {
    val clean = raw.trim().take(10)
    if (clean.length != 10) return null
    return runCatching { LocalDate.parse(clean) }.getOrNull()
}

private fun buildMonthGridCells(month: YearMonth): List<CalendarDayCell> {
    val today = LocalDate.now()
    val firstDay = month.atDay(1)
    val leading = firstDay.dayOfWeek.value - 1
    val start = firstDay.minusDays(leading.toLong())
    return List(42) { idx ->
        val d = start.plusDays(idx.toLong())
        CalendarDayCell(
            date = d,
            inMonth = d.year == month.year && d.monthValue == month.monthValue,
            isToday = d == today
        )
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ThemeCalendarDialog(
    initialDate: String,
    allowPerpetual: Boolean = true,
    onSelectDate: (String) -> Unit,
    onSelectPerpetual: (() -> Unit)? = null,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val parsedInitial = remember(initialDate) {
        val d = parseLocalDateOrNull(initialDate)
        if (d != null && d.year < 2099) d else LocalDate.now()
    }

    var selectedDate by remember { mutableStateOf(parsedInitial) }
    var currentMonth by remember { mutableStateOf(YearMonth.of(parsedInitial.year, parsedInitial.monthValue)) }
    var showYmGrid by remember { mutableStateOf(false) }

    val cells = remember(currentMonth) { buildMonthGridCells(currentMonth) }
    val yearChoices = remember(currentMonth.year) {
        val base = minOf(LocalDate.now().year - 2, currentMonth.year - 2)
        (base until (base + 12)).toList()
    }
    val weekHeaders = remember { listOf("一", "二", "三", "四", "五", "六", "日") }

    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false)
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp),
            shape = RoundedCornerShape(22.dp),
            color = theme.surfacePopover,
            border = BorderStroke(1.dp, theme.borderStrong),
            shadowElevation = 18.dp
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Icon(
                            imageVector = Icons.Default.CalendarMonth,
                            contentDescription = null,
                            tint = theme.accentForeground,
                            modifier = Modifier.size(18.dp)
                        )
                        Text(
                            text = "选择日期",
                            fontSize = 15.5.sp,
                            fontWeight = FontWeight.Bold,
                            color = theme.textPrimary
                        )
                    }
                    Text(
                        text = String.format(
                            Locale.US,
                            "%04d-%02d-%02d",
                            selectedDate.year,
                            selectedDate.monthValue,
                            selectedDate.dayOfMonth
                        ),
                        fontFamily = FontFamily.Monospace,
                        fontSize = 12.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.accentForeground
                    )
                }

                // Month & Year switcher header
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(12.dp))
                        .background(theme.surfaceInput)
                        .border(1.dp, theme.border, RoundedCornerShape(12.dp))
                        .padding(horizontal = 2.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    VesperIconButton(
                        icon = Icons.Default.ChevronLeft,
                        contentDescription = "上一月",
                        filled = false,
                        iconSize = 20.dp,
                        onClick = { currentMonth = currentMonth.minusMonths(1) }
                    )

                    Row(
                        modifier = Modifier
                            .sizeIn(minHeight = 44.dp)
                            .clip(RoundedCornerShape(10.dp))
                            .clickable { showYmGrid = !showYmGrid }
                            .padding(horizontal = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        Text(
                            text = "${currentMonth.year} 年 ${currentMonth.monthValue} 月",
                            fontFamily = FontFamily.Monospace,
                            fontSize = 13.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary
                        )
                        Icon(
                            imageVector = if (showYmGrid) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown,
                            contentDescription = null,
                            tint = theme.accentForeground,
                            modifier = Modifier.size(16.dp)
                        )
                    }

                    VesperIconButton(
                        icon = Icons.Default.ChevronRight,
                        contentDescription = "下一月",
                        filled = false,
                        iconSize = 20.dp,
                        onClick = { currentMonth = currentMonth.plusMonths(1) }
                    )
                }

                if (showYmGrid) {
                    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        FlowRow(
                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                            verticalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            yearChoices.forEach { yr ->
                                VesperSecondaryButton(
                                    text = "$yr",
                                    tone = if (yr == currentMonth.year) VesperButtonTone.ACCENT else VesperButtonTone.NEUTRAL,
                                    cornerRadius = 10.dp,
                                    minHeight = 36.dp,
                                    contentPadding = PaddingValues(horizontal = 11.dp, vertical = 6.dp),
                                    onClick = { currentMonth = YearMonth.of(yr, currentMonth.monthValue) }
                                )
                            }
                        }
                        FlowRow(
                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                            verticalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            (1..12).forEach { m ->
                                VesperSecondaryButton(
                                    text = "${m}月",
                                    tone = if (m == currentMonth.monthValue) VesperButtonTone.ACCENT else VesperButtonTone.NEUTRAL,
                                    cornerRadius = 10.dp,
                                    minHeight = 36.dp,
                                    contentPadding = PaddingValues(horizontal = 11.dp, vertical = 6.dp),
                                    onClick = {
                                        currentMonth = YearMonth.of(currentMonth.year, m)
                                        showYmGrid = false
                                    }
                                )
                            }
                        }
                    }
                } else {
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Row(modifier = Modifier.fillMaxWidth()) {
                            weekHeaders.forEach { label ->
                                Text(
                                    text = label,
                                    fontSize = 11.5.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = theme.textMuted,
                                    textAlign = TextAlign.Center,
                                    modifier = Modifier.weight(1f)
                                )
                            }
                        }

                        for (weekIdx in 0 until 6) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(4.dp)
                            ) {
                                for (dayIdx in 0 until 7) {
                                    val cell = cells[weekIdx * 7 + dayIdx]
                                    val isSelected = cell.date == selectedDate
                                    val bgColor = when {
                                        isSelected -> theme.accent
                                        cell.isToday -> theme.accentSoft
                                        else -> Color.Transparent
                                    }
                                    val borderStroke = when {
                                        isSelected -> BorderStroke(1.dp, theme.accent)
                                        cell.isToday -> BorderStroke(1.dp, theme.accentMuted)
                                        else -> null
                                    }
                                    val fgColor = when {
                                        isSelected -> theme.onAccent
                                        cell.isToday -> theme.accentForeground
                                        cell.inMonth -> theme.textPrimary
                                        else -> theme.textMuted.copy(alpha = 0.42f)
                                    }

                                    Box(
                                        modifier = Modifier
                                            .weight(1f)
                                            .aspectRatio(1.05f)
                                            .clip(CircleShape)
                                            .background(bgColor)
                                            .then(
                                                if (borderStroke != null) {
                                                    Modifier.border(borderStroke, CircleShape)
                                                } else {
                                                    Modifier
                                                }
                                            )
                                            .clickable {
                                                selectedDate = cell.date
                                                currentMonth = YearMonth.of(cell.date.year, cell.date.monthValue)
                                            },
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Text(
                                            text = cell.date.dayOfMonth.toString(),
                                            fontFamily = FontFamily.Monospace,
                                            fontSize = 12.5.sp,
                                            fontWeight = if (isSelected || cell.isToday) FontWeight.Bold else FontWeight.Normal,
                                            color = fgColor
                                        )
                                    }
                                }
                            }
                        }
                    }
                }

                // Quick date shortcuts
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    val jumpTo: (LocalDate) -> Unit = { d ->
                        selectedDate = d
                        currentMonth = YearMonth.of(d.year, d.monthValue)
                    }
                    listOf(
                        "今天" to { jumpTo(LocalDate.now()) },
                        "+1月" to { jumpTo(selectedDate.plusMonths(1)) },
                        "+1季" to { jumpTo(selectedDate.plusMonths(3)) },
                        "+1年" to { jumpTo(selectedDate.plusYears(1)) }
                    ).forEach { (label, action) ->
                        VesperFilterChip(
                            label = label,
                            selected = false,
                            onClick = action
                        )
                    }
                    if (allowPerpetual && onSelectPerpetual != null) {
                        VesperFilterChip(
                            label = "长期有效",
                            selected = false,
                            onClick = onSelectPerpetual
                        )
                    }
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    VesperSecondaryButton(
                        text = "取消",
                        minHeight = 48.dp,
                        onClick = onDismiss,
                        modifier = Modifier.weight(1f)
                    )
                    VesperPrimaryButton(
                        text = "确定日期",
                        minHeight = 48.dp,
                        onClick = {
                            onSelectDate(
                                String.format(
                                    Locale.US,
                                    "%04d-%02d-%02d",
                                    selectedDate.year,
                                    selectedDate.monthValue,
                                    selectedDate.dayOfMonth
                                )
                            )
                        },
                        modifier = Modifier.weight(1.3f)
                    )
                }
            }
        }
    }
}
