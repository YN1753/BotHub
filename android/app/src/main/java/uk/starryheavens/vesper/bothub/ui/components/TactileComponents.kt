package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.GrokBotGroupCluster

val NoFontPaddingStyle = PlatformTextStyle(includeFontPadding = false)

/**
 * Spring-physics tactile press modifier without ugly rectangular stock ripple.
 */
@OptIn(ExperimentalFoundationApi::class)
fun Modifier.tactileClickable(
    enabled: Boolean = true,
    pressedScale: Float = 0.968f,
    hapticOnTap: Boolean = false,
    role: Role? = Role.Button,
    onLongClick: (() -> Unit)? = null,
    onClick: () -> Unit
): Modifier = composed {
    val interactionSource = remember { MutableInteractionSource() }
    val isPressed by interactionSource.collectIsPressedAsState()
    val haptics = LocalHapticFeedback.current
    val scale by animateFloatAsState(
        targetValue = if (isPressed && enabled) pressedScale else 1f,
        animationSpec = spring(
            dampingRatio = 0.68f,
            stiffness = 560f
        ),
        label = "tactileScale"
    )
    val alpha by animateFloatAsState(
        targetValue = when {
            !enabled -> 0.45f
            isPressed -> 0.90f
            else -> 1f
        },
        animationSpec = tween(110),
        label = "tactileAlpha"
    )

    this
        .graphicsLayer {
            scaleX = scale
            scaleY = scale
            this.alpha = alpha
        }
        .then(
            if (onLongClick != null) {
                Modifier.combinedClickable(
                    interactionSource = interactionSource,
                    indication = null,
                    enabled = enabled,
                    role = role,
                    onLongClick = {
                        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                        onLongClick()
                    },
                    onClick = {
                        if (hapticOnTap) {
                            haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        }
                        onClick()
                    }
                )
            } else {
                Modifier.clickable(
                    interactionSource = interactionSource,
                    indication = null,
                    enabled = enabled,
                    role = role
                ) {
                    if (hapticOnTap) {
                        haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    }
                    onClick()
                }
            }
        )
}

/**
 * Directional 135-degree specular rim brush (bright top-left catchlight fading to ultra-subtle bottom-right).
 */
fun specularRimBrush(
    highlight: Color = Color(0x38FFF8EE),
    mid: Color = Color(0x12FFF8EE),
    shadow: Color = Color(0x06FFF8EE)
): Brush = Brush.linearGradient(
    colors = listOf(highlight, mid, shadow),
    start = Offset(0f, 0f),
    end = Offset(720f, 720f)
)

/**
 * Multi-stop vertical glass card with directional top-left specular catchlight and ambient depth.
 */
@Composable
fun SpecularGlassCard(
    modifier: Modifier = Modifier,
    cornerRadius: Dp = 18.dp,
    topColor: Color = Color(0xEB131B34),
    bottomColor: Color = Color(0xEB0A0F1F),
    accentTint: Color? = null,
    elevation: Dp = 6.dp,
    onClick: (() -> Unit)? = null,
    onLongClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit
) {
    val shape = RoundedCornerShape(cornerRadius)
    val rim = remember(accentTint) {
        if (accentTint != null) {
            specularRimBrush(
                highlight = accentTint.copy(alpha = 0.52f),
                mid = accentTint.copy(alpha = 0.16f),
                shadow = Color(0x0AFFF8EE)
            )
        } else {
            specularRimBrush()
        }
    }
    val bgBrush = remember(topColor, bottomColor, accentTint) {
        val effectiveTop = if (accentTint != null) {
            Color(
                red = (topColor.red * 0.86f + accentTint.red * 0.14f).coerceIn(0f, 1f),
                green = (topColor.green * 0.86f + accentTint.green * 0.14f).coerceIn(0f, 1f),
                blue = (topColor.blue * 0.86f + accentTint.blue * 0.14f).coerceIn(0f, 1f),
                alpha = topColor.alpha
            )
        } else {
            topColor
        }
        Brush.verticalGradient(listOf(effectiveTop, bottomColor))
    }

    val interactiveMod = if (onClick != null) {
        Modifier.tactileClickable(
            pressedScale = 0.982f,
            onLongClick = onLongClick,
            onClick = onClick
        )
    } else {
        Modifier
    }

    Column(
        modifier = modifier
            .then(interactiveMod)
            .shadow(
                elevation = elevation,
                shape = shape,
                ambientColor = Color(0x99000000),
                spotColor = Color(0xAA000000)
            )
            .clip(shape)
            .background(bgBrush, shape)
            .border(BorderStroke(0.85.dp, rim), shape),
        content = content
    )
}

/**
 * Bespoke Champagne-Gold Jewel Primary Button (replaces flat stock M3 Button).
 */
@Composable
fun TactilePrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    leadingIcon: ImageVector? = null,
    cornerRadius: Dp = 14.dp,
    minHeight: Dp = 46.dp,
    contentPadding: PaddingValues = PaddingValues(horizontal = 16.dp, vertical = 10.dp)
) {
    val shape = RoundedCornerShape(cornerRadius)
    val fillBrush = remember(enabled) {
        if (enabled) {
            Brush.verticalGradient(
                colors = listOf(
                    Color(0xFFFCE0C0),
                    Color(0xFFF2B880),
                    Color(0xFFE09A58)
                )
            )
        } else {
            Brush.verticalGradient(
                colors = listOf(
                    Color(0xFF283048),
                    Color(0xFF1B2236)
                )
            )
        }
    }
    val borderBrush = remember(enabled) {
        if (enabled) {
            Brush.verticalGradient(
                colors = listOf(
                    Color(0x99FFFFFF),
                    Color(0x24FFF8EE),
                    Color(0x337A481D)
                )
            )
        } else {
            SolidColor(Color(0x1FFFF8EE))
        }
    }

    Box(
        modifier = modifier
            .sizeIn(minHeight = minHeight)
            .tactileClickable(
                enabled = enabled,
                pressedScale = 0.966f,
                hapticOnTap = true,
                onClick = onClick
            )
            .shadow(
                elevation = if (enabled) 8.dp else 0.dp,
                shape = shape,
                ambientColor = Color(0x55F2B880),
                spotColor = Color(0x66F2B880)
            )
            .clip(shape)
            .background(fillBrush, shape)
            .border(BorderStroke(0.9.dp, borderBrush), shape)
            .padding(contentPadding),
        contentAlignment = Alignment.Center
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp, Alignment.CenterHorizontally)
        ) {
            if (leadingIcon != null) {
                Icon(
                    imageVector = leadingIcon,
                    contentDescription = null,
                    tint = if (enabled) Color(0xFF130E0A) else Color(0xFF64748B),
                    modifier = Modifier.size(16.dp)
                )
            }
            Text(
                text = text,
                style = TextStyle(
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Bold,
                    color = if (enabled) Color(0xFF130E0A) else Color(0xFF64748B),
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

/**
 * Bespoke Specular Slate / Ruby Secondary Button (replaces stock OutlinedButton / flat Surface buttons).
 */
@Composable
fun TactileSecondaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    danger: Boolean = false,
    accent: Boolean = false,
    leadingIcon: ImageVector? = null,
    cornerRadius: Dp = 14.dp,
    minHeight: Dp = 44.dp,
    contentPadding: PaddingValues = PaddingValues(horizontal = 14.dp, vertical = 9.dp)
) {
    val shape = RoundedCornerShape(cornerRadius)
    val bgBrush = remember(danger, accent) {
        when {
            danger -> Brush.verticalGradient(listOf(Color(0x30F43F5E), Color(0x18BE123C)))
            accent -> Brush.verticalGradient(listOf(Color(0x2EF2B880), Color(0x18C68850)))
            else -> Brush.verticalGradient(listOf(Color(0xFF182240), Color(0xFF0E1529)))
        }
    }
    val rim = remember(danger, accent) {
        when {
            danger -> specularRimBrush(Color(0x70FB7185), Color(0x30FB7185), Color(0x14FB7185))
            accent -> specularRimBrush(Color(0x78F2B880), Color(0x30F2B880), Color(0x14F2B880))
            else -> specularRimBrush(Color(0x38FFF8EE), Color(0x16FFF8EE), Color(0x08FFF8EE))
        }
    }
    val fgColor = when {
        !enabled -> Color(0xFF64748B)
        danger -> Color(0xFFFDA4AF)
        accent -> Color(0xFFFFE4C4)
        else -> Color(0xFFE6ECF8)
    }

    Box(
        modifier = modifier
            .sizeIn(minHeight = minHeight)
            .tactileClickable(
                enabled = enabled,
                pressedScale = 0.968f,
                onClick = onClick
            )
            .clip(shape)
            .background(bgBrush, shape)
            .border(BorderStroke(0.85.dp, rim), shape)
            .padding(contentPadding),
        contentAlignment = Alignment.Center
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(5.dp, Alignment.CenterHorizontally)
        ) {
            if (leadingIcon != null) {
                Icon(
                    imageVector = leadingIcon,
                    contentDescription = null,
                    tint = fgColor,
                    modifier = Modifier.size(15.dp)
                )
            }
            Text(
                text = text,
                style = TextStyle(
                    fontSize = 13.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = fgColor,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

/**
 * Custom 46x26dp iOS / WeChat Sliding Capsule Toggle (replaces bulky stock M3 Switch everywhere).
 */
@Composable
fun TactileToggle(
    checked: Boolean,
    onCheckedChange: ((Boolean) -> Unit)?,
    enabled: Boolean = true,
    modifier: Modifier = Modifier
) {
    val haptics = LocalHapticFeedback.current
    val thumbOffset by animateDpAsState(
        targetValue = if (checked) 20.dp else 0.dp,
        animationSpec = spring(
            dampingRatio = 0.72f,
            stiffness = 580f
        ),
        label = "toggleThumbOffset"
    )
    val trackTop by animateColorAsState(
        targetValue = if (checked) Color(0xFFF6C692) else Color(0xFF12192E),
        animationSpec = tween(180),
        label = "toggleTrackTop"
    )
    val trackBottom by animateColorAsState(
        targetValue = if (checked) Color(0xFFDF9855) else Color(0xFF0A0F1E),
        animationSpec = tween(180),
        label = "toggleTrackBottom"
    )
    val borderColor by animateColorAsState(
        targetValue = if (checked) Color(0x66FFE8CC) else Color(0x28FFF8EE),
        animationSpec = tween(180),
        label = "toggleBorder"
    )

    val clickMod = if (onCheckedChange != null) {
        Modifier.clickable(
            interactionSource = remember { MutableInteractionSource() },
            indication = null,
            enabled = enabled,
            role = Role.Switch
        ) {
            haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
            onCheckedChange(!checked)
        }
    } else {
        Modifier
    }

    Box(
        modifier = modifier
            .width(46.dp)
            .height(26.dp)
            .graphicsLayer { alpha = if (enabled) 1f else 0.45f }
            .clip(CircleShape)
            .background(Brush.verticalGradient(listOf(trackTop, trackBottom)), CircleShape)
            .border(0.8.dp, borderColor, CircleShape)
            .then(clickMod)
            .padding(2.dp),
        contentAlignment = Alignment.CenterStart
    ) {
        Box(
            modifier = Modifier
                .offset(x = thumbOffset)
                .size(22.dp)
                .shadow(
                    elevation = 3.dp,
                    shape = CircleShape,
                    ambientColor = Color(0x88000000),
                    spotColor = Color(0xAA000000)
                )
                .background(
                    brush = Brush.verticalGradient(
                        colors = if (checked) {
                            listOf(Color(0xFFFFFFFF), Color(0xFFFFF4E6))
                        } else {
                            listOf(Color(0xFFE2E8F0), Color(0xFF94A3B8))
                        }
                    ),
                    shape = CircleShape
                )
        )
    }
}

/**
 * Recessed Pill Search Bar (WeChat / QQ / Telegram inset dark search capsule).
 */
@Composable
fun RecessedSearchBar(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier
) {
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(13.dp)
    val borderColor by animateColorAsState(
        targetValue = if (focused) Color(0x66F2B880) else Color(0x16FFF8EE),
        animationSpec = tween(150),
        label = "searchBorder"
    )

    Box(
        modifier = modifier
            .fillMaxWidth()
            .sizeIn(minHeight = 38.dp)
            .clip(shape)
            .background(
                Brush.verticalGradient(
                    listOf(Color(0xEB070B16), Color(0xEB0C1224))
                ),
                shape
            )
            .border(0.8.dp, borderColor, shape)
            .padding(horizontal = 12.dp, vertical = 8.dp),
        contentAlignment = Alignment.CenterStart
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            Icon(
                imageVector = Icons.Default.Search,
                contentDescription = null,
                tint = if (focused) Color(0xFFF2B880) else Color(0xFF64748B),
                modifier = Modifier.size(16.dp)
            )
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                singleLine = true,
                textStyle = TextStyle(
                    color = Color(0xFFFFF8EE),
                    fontSize = 13.5.sp,
                    platformStyle = NoFontPaddingStyle
                ),
                cursorBrush = SolidColor(Color(0xFFF2B880)),
                modifier = Modifier
                    .weight(1f)
                    .onFocusChanged { focused = it.isFocused },
                decorationBox = { inner ->
                    Box(contentAlignment = Alignment.CenterStart) {
                        if (value.isEmpty()) {
                            Text(
                                text = placeholder,
                                style = TextStyle(
                                    fontSize = 13.sp,
                                    color = Color(0xFF5A6A85),
                                    platformStyle = NoFontPaddingStyle
                                ),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                        inner()
                    }
                }
            )
            if (value.isNotEmpty()) {
                Box(
                    modifier = Modifier
                        .size(18.dp)
                        .clip(CircleShape)
                        .background(Color(0x26FFFFFF))
                        .clickable { onValueChange("") },
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Default.Close,
                        contentDescription = "清空",
                        tint = Color(0xFFCBD5E1),
                        modifier = Modifier.size(11.dp)
                    )
                }
            }
        }
    }
}

/**
 * Spirit-tinted Duotone Squircle Avatar (replaces dark gray wire-bordered avatar boxes).
 */
@Composable
fun SpiritSquircleAvatar(
    bot: BotItem?,
    allBots: List<BotItem>,
    avatarStyleOverride: String? = null,
    state: String = bot?.state ?: "idle",
    containerSize: Dp = 48.dp,
    cornerRadius: Dp = 14.dp,
    showPresenceDot: Boolean = false,
    interactive: Boolean = false,
    seedKey: String = bot?.id ?: "spirit",
    onClick: (() -> Unit)? = null,
    modifier: Modifier = Modifier
) {
    val styleRaw = (avatarStyleOverride ?: bot?.avatarStyle ?: "butler:gold").lowercase()
    val isGroup = bot?.isGroup == true

    val (bgTop, bgBottom, rimAccent) = remember(styleRaw, isGroup) {
        when {
            isGroup -> Triple(Color(0xFF1A203D), Color(0xFF0E1326), Color(0x4DF2B880))
            styleRaw.contains("emerald") || styleRaw.contains("radar") ->
                Triple(Color(0xFF112928), Color(0xFF0A1719), Color(0x4734D399))
            styleRaw.contains("indigo") || styleRaw.contains("sentinel") || styleRaw.contains("cyan") ->
                Triple(Color(0xFF162142), Color(0xFF0C1226), Color(0x47818CF8))
            styleRaw.contains("violet") || styleRaw.contains("swarm") ->
                Triple(Color(0xFF23183E), Color(0xFF120D24), Color(0x47C084FC))
            styleRaw.contains("rose") || styleRaw.contains("critic") || styleRaw.contains("amber") ->
                Triple(Color(0xFF2B1920), Color(0xFF150D13), Color(0x47FB7185))
            else ->
                Triple(Color(0xFF261D18), Color(0xFF130F12), Color(0x4DF2B880))
        }
    }

    val shape = RoundedCornerShape(cornerRadius)
    val innerSize = containerSize * 0.86f
    val clickMod = if (onClick != null) {
        Modifier.tactileClickable(pressedScale = 0.94f, onClick = onClick)
    } else {
        Modifier
    }

    Box(modifier = modifier.size(containerSize)) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .then(clickMod)
                .shadow(
                    elevation = 4.dp,
                    shape = shape,
                    ambientColor = Color(0x77000000),
                    spotColor = Color(0x88000000)
                )
                .clip(shape)
                .background(Brush.verticalGradient(listOf(bgTop, bgBottom)), shape)
                .border(
                    BorderStroke(
                        0.75.dp,
                        specularRimBrush(rimAccent, rimAccent.copy(alpha = 0.14f), Color(0x08FFFFFF))
                    ),
                    shape
                ),
            contentAlignment = Alignment.Center
        ) {
            if (isGroup && bot != null) {
                GrokBotGroupCluster(
                    members = bot.resolveGroupMembers(allBots),
                    fallbackStyle = bot.avatarStyle,
                    state = state,
                    size = innerSize,
                    interactive = interactive
                )
            } else {
                GrokBotAvatar(
                    avatarStyle = avatarStyleOverride ?: bot?.avatarStyle ?: "butler:gold",
                    state = state,
                    size = innerSize,
                    interactive = interactive,
                    seedKey = seedKey
                )
            }
        }

        if (showPresenceDot && !isGroup) {
            val dotColor = when (state) {
                "working" -> Color(0xFFF2B880)
                "thinking" -> Color(0xFFFBBF24)
                "blocked" -> Color(0xFFFB7185)
                else -> Color(0xFF34D399)
            }
            Box(
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .offset(x = 2.dp, y = 2.dp)
                    .size(12.dp)
                    .background(Color(0xFF050811), CircleShape)
                    .padding(2.dp)
                    .background(dotColor, CircleShape)
            )
        }
    }
}

/**
 * Unread Red/Champagne Pill Badge immune to OEM handwritten font vertical stretching.
 */
@Composable
fun CrispUnreadBadge(
    count: Int,
    containerColor: Color = Color(0xFFF43F5E),
    contentColor: Color = Color.White,
    modifier: Modifier = Modifier
) {
    if (count <= 0) return
    Box(
        modifier = modifier
            .shadow(3.dp, RoundedCornerShape(999.dp), ambientColor = containerColor.copy(alpha = 0.5f))
            .defaultMinSize(minWidth = 17.dp, minHeight = 17.dp)
            .height(17.dp)
            .background(containerColor, RoundedCornerShape(999.dp))
            .border(0.6.dp, Color(0x33FFFFFF), RoundedCornerShape(999.dp))
            .padding(horizontal = 4.5.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = if (count > 99) "99+" else count.toString(),
            style = TextStyle(
                fontSize = 10.sp,
                lineHeight = 11.sp,
                fontFamily = FontFamily.Monospace,
                fontWeight = FontWeight.Bold,
                color = contentColor,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false
        )
    }
}

/**
 * Bulletproof non-wrapping micro tag pill (never squeezes vertically into 1-char columns).
 */
@Composable
fun CrispTagPill(
    text: String,
    bgColor: Color = Color(0x16FFFFFF),
    borderColor: Color = Color(0x24FFF8EE),
    textColor: Color = Color(0xFF94A3B8),
    modifier: Modifier = Modifier
) {
    val shape = RoundedCornerShape(5.dp)
    Box(
        modifier = modifier
            .clip(shape)
            .background(bgColor, shape)
            .border(0.5.dp, borderColor, shape)
            .padding(horizontal = 5.5.dp, vertical = 1.5.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = text,
            style = TextStyle(
                fontSize = 9.5.sp,
                fontWeight = FontWeight.Medium,
                color = textColor,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false
        )
    }
}

data class GlassMenuAction(
    val label: String,
    val icon: ImageVector? = null,
    val iconTint: Color = Color(0xFFFFE2BE),
    val danger: Boolean = false,
    val onClick: () -> Unit
)

/**
 * Custom Floating Rounded Specular Glass Popover (replaces stock rectangular M3 DropdownMenu).
 */
@Composable
fun FloatingGlassPopover(
    expanded: Boolean,
    onDismissRequest: () -> Unit,
    items: List<GlassMenuAction>,
    alignment: Alignment = Alignment.TopEnd,
    offsetX: Dp = 0.dp,
    offsetY: Dp = 42.dp
) {
    if (!expanded) return

    val density = LocalDensity.current
    val pxOffset = remember(density, offsetX, offsetY) {
        with(density) { IntOffset(offsetX.roundToPx(), offsetY.roundToPx()) }
    }
    var animateIn by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) {
        animateIn = true
    }

    Popup(
        alignment = alignment,
        offset = pxOffset,
        onDismissRequest = onDismissRequest,
        properties = PopupProperties(focusable = true)
    ) {
        AnimatedVisibility(
            visible = animateIn,
            enter = fadeIn(tween(130)) + scaleIn(
                initialScale = 0.90f,
                transformOrigin = TransformOrigin(0.9f, 0f),
                animationSpec = spring(dampingRatio = Spring.DampingRatioNoBouncy, stiffness = 620f)
            ),
            exit = fadeOut(tween(90)) + scaleOut(targetScale = 0.94f)
        ) {
            val shape = RoundedCornerShape(18.dp)
            Column(
                modifier = Modifier
                    .padding(horizontal = 8.dp, vertical = 4.dp)
                    .widthIn(min = 172.dp, max = 224.dp)
                    .shadow(
                        elevation = 18.dp,
                        shape = shape,
                        ambientColor = Color(0xDD000000),
                        spotColor = Color(0xEE000000)
                    )
                    .clip(shape)
                    .background(
                        Brush.verticalGradient(
                            listOf(Color(0xFA16203D), Color(0xFA0C1225))
                        ),
                        shape
                    )
                    .border(
                        BorderStroke(
                            0.9.dp,
                            specularRimBrush(Color(0x4DFFF8EE), Color(0x1CFFF8EE), Color(0x0AFFF8EE))
                        ),
                        shape
                    )
                    .padding(vertical = 5.dp)
            ) {
                items.forEachIndexed { idx, action ->
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable {
                                onDismissRequest()
                                action.onClick()
                            }
                            .padding(horizontal = 15.dp, vertical = 11.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(11.dp)
                    ) {
                        if (action.icon != null) {
                            Box(
                                modifier = Modifier
                                    .size(26.dp)
                                    .clip(RoundedCornerShape(8.dp))
                                    .background(action.iconTint.copy(alpha = 0.14f)),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = action.icon,
                                    contentDescription = null,
                                    tint = if (action.danger) Color(0xFFFB7185) else action.iconTint,
                                    modifier = Modifier.size(15.dp)
                                )
                            }
                        }
                        Text(
                            text = action.label,
                            style = TextStyle(
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = if (action.danger) Color(0xFFFDA4AF) else Color(0xFFFFF8EE),
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    if (idx < items.lastIndex) {
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x18FFF8EE),
                            modifier = Modifier.padding(
                                start = if (action.icon != null) 52.dp else 14.dp,
                                end = 14.dp
                            )
                        )
                    }
                }
            }
        }
    }
}

/**
 * Tactile Filter Pill Chip with smooth specular selection state.
 */
@Composable
fun TactileFilterChip(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val shape = RoundedCornerShape(999.dp)
    val bgBrush = remember(selected) {
        if (selected) {
            Brush.verticalGradient(listOf(Color(0x36F2B880), Color(0x1CC88A4E)))
        } else {
            Brush.verticalGradient(listOf(Color(0xCC111930), Color(0xCC0A0F1E)))
        }
    }
    val rim = remember(selected) {
        if (selected) {
            specularRimBrush(Color(0x88F2B880), Color(0x40F2B880), Color(0x1AF2B880))
        } else {
            specularRimBrush(Color(0x28FFF8EE), Color(0x10FFF8EE), Color(0x06FFF8EE))
        }
    }

    Box(
        modifier = modifier
            .sizeIn(minHeight = 34.dp)
            .tactileClickable(pressedScale = 0.95f, hapticOnTap = true, onClick = onClick)
            .clip(shape)
            .background(bgBrush, shape)
            .border(BorderStroke(0.85.dp, rim), shape)
            .padding(horizontal = 14.dp, vertical = 6.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = label,
            style = TextStyle(
                fontSize = 12.sp,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                color = if (selected) Color(0xFFFFE4C4) else Color(0xFF94A3B8),
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false
        )
    }
}

/**
 * Subtle Top-Highlight Hairline Divider.
 */
@Composable
fun SpecularHairline(modifier: Modifier = Modifier) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(0.6.dp)
            .drawWithContent {
                drawRect(
                    brush = Brush.horizontalGradient(
                        listOf(
                            Color(0x05FFF8EE),
                            Color(0x26FFF8EE),
                            Color(0x05FFF8EE)
                        )
                    )
                )
            }
    )
}
