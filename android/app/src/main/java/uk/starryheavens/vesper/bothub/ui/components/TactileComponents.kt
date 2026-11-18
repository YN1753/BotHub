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
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
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
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

val NoFontPaddingStyle = PlatformTextStyle(includeFontPadding = false)

private val UnsetColor = Color.Unspecified

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
 * Multi-stop vertical glass card with directional top-left specular catchlight and theme-unified depth.
 */
@Composable
fun SpecularGlassCard(
    modifier: Modifier = Modifier,
    cornerRadius: Dp = 18.dp,
    topColor: Color = UnsetColor,
    bottomColor: Color = UnsetColor,
    accentTint: Color? = null,
    elevation: Dp = 6.dp,
    onClick: (() -> Unit)? = null,
    onLongClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit
) {
    val theme = BotHubThemeTokens
    val resolvedTop = if (topColor != UnsetColor) topColor else theme.cardTop
    val resolvedBottom = if (bottomColor != UnsetColor) bottomColor else theme.cardBottom
    val shape = RoundedCornerShape(cornerRadius)
    val rim = remember(accentTint, theme.id) {
        if (accentTint != null) {
            specularRimBrush(
                highlight = accentTint.copy(alpha = if (theme.isLight) 0.42f else 0.52f),
                mid = accentTint.copy(alpha = 0.16f),
                shadow = theme.rimSubtle
            )
        } else {
            specularRimBrush(theme.rimHighlight, theme.rimMid, theme.rimSubtle)
        }
    }
    val bgBrush = remember(resolvedTop, resolvedBottom, accentTint) {
        val effectiveTop = if (accentTint != null) {
            Color(
                red = (resolvedTop.red * 0.88f + accentTint.red * 0.12f).coerceIn(0f, 1f),
                green = (resolvedTop.green * 0.88f + accentTint.green * 0.12f).coerceIn(0f, 1f),
                blue = (resolvedTop.blue * 0.88f + accentTint.blue * 0.12f).coerceIn(0f, 1f),
                alpha = resolvedTop.alpha
            )
        } else {
            resolvedTop
        }
        Brush.verticalGradient(listOf(effectiveTop, resolvedBottom))
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

    val ambientShadow = if (theme.isLight) Color(0x1C0F172A) else Color(0x99000000)
    val spotShadow = if (theme.isLight) Color(0x280F172A) else Color(0xAA000000)

    Column(
        modifier = modifier
            .then(interactiveMod)
            .shadow(
                elevation = if (theme.isLight) (elevation * 0.65f) else elevation,
                shape = shape,
                ambientColor = ambientShadow,
                spotColor = spotShadow
            )
            .clip(shape)
            .background(bgBrush, shape)
            .border(BorderStroke(0.85.dp, rim), shape),
        content = content
    )
}

/**
 * Theme-Adaptive Jewel Primary Button.
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
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(cornerRadius)
    val fillBrush = remember(enabled, theme.id) {
        if (enabled) {
            if (theme.isLight) {
                Brush.verticalGradient(
                    colors = listOf(theme.accent, theme.accentDeep)
                )
            } else {
                Brush.verticalGradient(
                    colors = listOf(theme.accentLight, theme.accent, theme.accentDeep)
                )
            }
        } else {
            Brush.verticalGradient(
                colors = listOf(theme.inputTop, theme.inputBottom)
            )
        }
    }
    val borderBrush = remember(enabled, theme.id) {
        if (enabled) {
            Brush.verticalGradient(
                colors = listOf(
                    Color(0x80FFFFFF),
                    theme.accent.copy(alpha = 0.35f),
                    theme.accentDeep.copy(alpha = 0.45f)
                )
            )
        } else {
            SolidColor(theme.divider)
        }
    }
    val fgColor = if (enabled) theme.onAccent else theme.textMuted

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
                elevation = if (enabled) (if (theme.isLight) 4.dp else 8.dp) else 0.dp,
                shape = shape,
                ambientColor = theme.accent.copy(alpha = 0.35f),
                spotColor = theme.accent.copy(alpha = 0.42f)
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
                    tint = fgColor,
                    modifier = Modifier.size(16.dp)
                )
            }
            Text(
                text = text,
                style = TextStyle(
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Bold,
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
 * Theme-Adaptive Specular Secondary Button.
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
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(cornerRadius)
    val bgBrush = remember(danger, accent, theme.id) {
        when {
            danger -> Brush.verticalGradient(
                listOf(
                    Color(0xFFF43F5E).copy(alpha = if (theme.isLight) 0.12f else 0.22f),
                    Color(0xFFBE123C).copy(alpha = if (theme.isLight) 0.06f else 0.12f)
                )
            )
            accent -> Brush.verticalGradient(
                listOf(
                    theme.accent.copy(alpha = if (theme.isLight) 0.14f else 0.20f),
                    theme.accentDeep.copy(alpha = if (theme.isLight) 0.08f else 0.11f)
                )
            )
            else -> Brush.verticalGradient(listOf(theme.secondaryBtnTop, theme.secondaryBtnBottom))
        }
    }
    val rim = remember(danger, accent, theme.id) {
        when {
            danger -> specularRimBrush(Color(0x70FB7185), Color(0x30FB7185), Color(0x14FB7185))
            accent -> specularRimBrush(
                theme.accent.copy(alpha = 0.50f),
                theme.accent.copy(alpha = 0.22f),
                theme.accent.copy(alpha = 0.08f)
            )
            else -> specularRimBrush(theme.rimHighlight, theme.rimMid, theme.rimSubtle)
        }
    }
    val fgColor = when {
        !enabled -> theme.textMuted
        danger -> if (theme.isLight) Color(0xFFE11D48) else Color(0xFFFDA4AF)
        accent -> if (theme.isLight) theme.accentDeep else theme.accentLight
        else -> theme.textPrimary
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
 * Custom 46x26dp Sliding Capsule Toggle.
 */
@Composable
fun TactileToggle(
    checked: Boolean,
    onCheckedChange: ((Boolean) -> Unit)?,
    enabled: Boolean = true,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
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
        targetValue = if (checked) theme.accent else theme.inputTop,
        animationSpec = tween(180),
        label = "toggleTrackTop"
    )
    val trackBottom by animateColorAsState(
        targetValue = if (checked) theme.accentDeep else theme.inputBottom,
        animationSpec = tween(180),
        label = "toggleTrackBottom"
    )
    val borderColor by animateColorAsState(
        targetValue = if (checked) theme.accent.copy(alpha = 0.55f) else theme.rimHighlight,
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
                    ambientColor = Color(0x55000000),
                    spotColor = Color(0x77000000)
                )
                .background(
                    brush = Brush.verticalGradient(
                        colors = listOf(Color(0xFFFFFFFF), Color(0xFFF1F5F9))
                    ),
                    shape = CircleShape
                )
        )
    }
}

/**
 * Recessed Pill Search Bar.
 */
@Composable
fun RecessedSearchBar(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(13.dp)
    val borderColor by animateColorAsState(
        targetValue = if (focused) theme.accent.copy(alpha = 0.65f) else theme.divider,
        animationSpec = tween(150),
        label = "searchBorder"
    )

    Box(
        modifier = modifier
            .fillMaxWidth()
            .sizeIn(minHeight = 38.dp)
            .clip(shape)
            .background(
                Brush.verticalGradient(listOf(theme.inputTop, theme.inputBottom)),
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
                tint = if (focused) theme.accent else theme.textMuted,
                modifier = Modifier.size(16.dp)
            )
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                singleLine = true,
                textStyle = TextStyle(
                    color = theme.textPrimary,
                    fontSize = 13.5.sp,
                    platformStyle = NoFontPaddingStyle
                ),
                cursorBrush = SolidColor(theme.accent),
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
                                    color = theme.textMuted,
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
                        .background(theme.textMuted.copy(alpha = 0.22f))
                        .clickable { onValueChange("") },
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Default.Close,
                        contentDescription = "清空",
                        tint = theme.textPrimary,
                        modifier = Modifier.size(11.dp)
                    )
                }
            }
        }
    }
}

/**
 * Spirit-tinted Duotone Squircle Avatar.
 */
@Composable
fun SpiritSquircleAvatar(
    bot: BotItem?,
    allBots: List<BotItem>,
    avatarStyleOverride: String? = null,
    state: String = bot?.state ?: "idle",
    containerSize: Dp = 48.dp,
    @Suppress("UNUSED_PARAMETER")
    cornerRadius: Dp = 14.dp,
    showPresenceDot: Boolean = false,
    interactive: Boolean = false,
    seedKey: String = bot?.id ?: "spirit",
    onClick: (() -> Unit)? = null,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val isGroup = bot?.isGroup == true
    val innerSize = containerSize
    val clickMod = if (onClick != null) {
        Modifier.tactileClickable(pressedScale = 0.94f, onClick = onClick)
    } else {
        Modifier
    }

    Box(modifier = modifier.size(containerSize)) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .then(clickMod),
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
                "working" -> theme.accent
                "thinking" -> Color(0xFFFBBF24)
                "blocked" -> Color(0xFFFB7185)
                else -> Color(0xFF10B981)
            }
            Box(
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .offset(x = 2.dp, y = 2.dp)
                    .size(12.dp)
                    .background(theme.bgBase, CircleShape)
                    .padding(2.dp)
                    .background(dotColor, CircleShape)
            )
        }
    }
}

/**
 * Unread Red/Accent Pill Badge immune to OEM handwritten font vertical stretching.
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
 * Bulletproof non-wrapping micro tag pill.
 */
@Composable
fun CrispTagPill(
    text: String,
    bgColor: Color = UnsetColor,
    borderColor: Color = UnsetColor,
    textColor: Color = UnsetColor,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val resolvedBg = if (bgColor != UnsetColor) bgColor else theme.accent.copy(alpha = 0.14f)
    val resolvedBorder = if (borderColor != UnsetColor) borderColor else theme.accent.copy(alpha = 0.36f)
    val resolvedText = if (textColor != UnsetColor) textColor else (if (theme.isLight) theme.accentDeep else theme.accentLight)
    val shape = RoundedCornerShape(5.dp)
    Box(
        modifier = modifier
            .clip(shape)
            .background(resolvedBg, shape)
            .border(0.5.dp, resolvedBorder, shape)
            .padding(horizontal = 5.5.dp, vertical = 1.5.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = text,
            style = TextStyle(
                fontSize = 9.5.sp,
                fontWeight = FontWeight.Medium,
                color = resolvedText,
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
    val iconTint: Color = UnsetColor,
    val danger: Boolean = false,
    val onClick: () -> Unit
)

/**
 * Theme-Unified Floating Rounded Specular Glass Popover (replaces rectangular stock M3 DropdownMenu).
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

    val theme = BotHubThemeTokens
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
                        elevation = if (theme.isLight) 10.dp else 18.dp,
                        shape = shape,
                        ambientColor = if (theme.isLight) Color(0x330F172A) else Color(0xDD000000),
                        spotColor = if (theme.isLight) Color(0x400F172A) else Color(0xEE000000)
                    )
                    .clip(shape)
                    .background(
                        Brush.verticalGradient(listOf(theme.popoverTop, theme.popoverBottom)),
                        shape
                    )
                    .border(
                        BorderStroke(
                            0.9.dp,
                            specularRimBrush(theme.rimHighlight, theme.rimMid, theme.rimSubtle)
                        ),
                        shape
                    )
                    .padding(vertical = 5.dp)
            ) {
                items.forEachIndexed { idx, action ->
                    val effectiveTint = when {
                        action.danger -> if (theme.isLight) Color(0xFFE11D48) else Color(0xFFFB7185)
                        action.iconTint != UnsetColor -> {
                            if (theme.isLight && action.iconTint.luminanceApprox() > 0.72f) {
                                theme.accent
                            } else {
                                action.iconTint
                            }
                        }
                        else -> theme.accent
                    }
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
                                    .background(effectiveTint.copy(alpha = 0.14f)),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = action.icon,
                                    contentDescription = null,
                                    tint = effectiveTint,
                                    modifier = Modifier.size(15.dp)
                                )
                            }
                        }
                        Text(
                            text = action.label,
                            style = TextStyle(
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = if (action.danger) effectiveTint else theme.textPrimary,
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
                            color = theme.divider,
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

private fun Color.luminanceApprox(): Float = 0.299f * red + 0.587f * green + 0.114f * blue

/**
 * Theme-Unified Floating Glass Snackbar Host (replaces stark white Material 3 stock Snackbar).
 */
@Composable
fun BotHubSnackbarHost(
    hostState: SnackbarHostState,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    SnackbarHost(
        hostState = hostState,
        modifier = modifier
    ) { data ->
        val msg = data.visuals.message
        val isError = msg.contains("失败") || msg.contains("异常") || msg.contains("错误") || msg.contains("无效")
        val statusColor = if (isError) Color(0xFFF43F5E) else theme.accent
        val shape = RoundedCornerShape(16.dp)

        Row(
            modifier = Modifier
                .padding(horizontal = 18.dp, vertical = 10.dp)
                .shadow(
                    elevation = if (theme.isLight) 8.dp else 14.dp,
                    shape = shape,
                    ambientColor = if (theme.isLight) Color(0x2E0F172A) else Color(0xCC000000),
                    spotColor = if (theme.isLight) Color(0x380F172A) else Color(0xDD000000)
                )
                .clip(shape)
                .background(
                    Brush.verticalGradient(listOf(theme.popoverTop, theme.popoverBottom)),
                    shape
                )
                .border(
                    BorderStroke(
                        0.9.dp,
                        specularRimBrush(
                            statusColor.copy(alpha = 0.55f),
                            theme.rimMid,
                            theme.rimSubtle
                        )
                    ),
                    shape
                )
                .clickable { data.dismiss() }
                .padding(horizontal = 15.dp, vertical = 11.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(7.dp)
                    .background(statusColor, CircleShape)
            )
            Text(
                text = msg,
                style = TextStyle(
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textPrimary,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f, fill = false)
            )
        }
    }
}

/**
 * Tactile Filter Pill Chip with theme-adaptive selection state.
 */
@Composable
fun TactileFilterChip(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(999.dp)
    val bgBrush = remember(selected, theme.id) {
        if (selected) {
            Brush.verticalGradient(
                listOf(
                    theme.accent.copy(alpha = if (theme.isLight) 0.16f else 0.22f),
                    theme.accentDeep.copy(alpha = if (theme.isLight) 0.08f else 0.12f)
                )
            )
        } else {
            Brush.verticalGradient(listOf(theme.cardTop, theme.cardBottom))
        }
    }
    val rim = remember(selected, theme.id) {
        if (selected) {
            specularRimBrush(
                theme.accent.copy(alpha = 0.58f),
                theme.accent.copy(alpha = 0.26f),
                theme.accent.copy(alpha = 0.10f)
            )
        } else {
            specularRimBrush(theme.rimHighlight, theme.rimMid, theme.rimSubtle)
        }
    }
    val fgColor = if (selected) {
        if (theme.isLight) theme.accentDeep else theme.accentLight
    } else {
        theme.textMuted
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
                color = fgColor,
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
    val theme = BotHubThemeTokens
    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(0.6.dp)
            .drawWithContent {
                drawRect(
                    brush = Brush.horizontalGradient(
                        listOf(
                            theme.divider.copy(alpha = 0.25f),
                            theme.divider,
                            theme.divider.copy(alpha = 0.25f)
                        )
                    )
                )
            }
    )
}

/**
 * Recessed Capsule Segmented Control with theme-adaptive indicator.
 */
@Composable
fun TactileSegmentedControl(
    options: List<String>,
    selectedIndex: Int,
    onSelect: (Int) -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val outerShape = RoundedCornerShape(14.dp)
    val pillShape = RoundedCornerShape(11.dp)

    Row(
        modifier = modifier
            .fillMaxWidth()
            .clip(outerShape)
            .background(theme.inputTop, outerShape)
            .border(
                BorderStroke(
                    0.85.dp,
                    specularRimBrush(theme.rimHighlight, theme.rimMid, Color.Transparent)
                ),
                outerShape
            )
            .padding(3.5.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        options.forEachIndexed { idx, label ->
            val selected = idx == selectedIndex
            val textColor by animateColorAsState(
                targetValue = if (selected) theme.textPrimary else theme.textMuted,
                animationSpec = tween(140),
                label = "segTextColor"
            )
            Box(
                modifier = Modifier
                    .weight(1f)
                    .sizeIn(minHeight = 35.dp)
                    .tactileClickable(
                        pressedScale = 0.96f,
                        hapticOnTap = true,
                        onClick = { if (!selected) onSelect(idx) }
                    )
                    .clip(pillShape)
                    .then(
                        if (selected) {
                            Modifier
                                .background(
                                    Brush.verticalGradient(
                                        listOf(theme.popoverTop, theme.popoverBottom)
                                    ),
                                    pillShape
                                )
                                .border(
                                    BorderStroke(
                                        0.85.dp,
                                        specularRimBrush(
                                            theme.accent.copy(alpha = 0.48f),
                                            theme.accent.copy(alpha = 0.18f),
                                            theme.rimSubtle
                                        )
                                    ),
                                    pillShape
                                )
                        } else {
                            Modifier
                        }
                    )
                    .padding(horizontal = 10.dp, vertical = 6.dp),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = label,
                    style = TextStyle(
                        fontSize = 13.sp,
                        fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                        color = textColor,
                        platformStyle = NoFontPaddingStyle
                    ),
                    maxLines = 1,
                    softWrap = false,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}
