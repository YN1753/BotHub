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
import androidx.compose.foundation.layout.Spacer
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
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircleOutline
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.ErrorOutline
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
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.onFocusChanged
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
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
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

enum class VesperButtonTone {
    NEUTRAL,
    ACCENT,
    DANGER
}

/**
 * Spring-physics press feedback modifier using GPU graphicsLayer for 120Hz fluidity.
 */
@OptIn(ExperimentalFoundationApi::class)
fun Modifier.vesperPressable(
    enabled: Boolean = true,
    pressedScale: Float = 0.975f,
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
            dampingRatio = 0.74f,
            stiffness = 620f
        ),
        label = "vesperScale"
    )
    val alpha by animateFloatAsState(
        targetValue = when {
            !enabled -> 0.44f
            isPressed -> 0.88f
            else -> 1f
        },
        animationSpec = tween(95),
        label = "vesperAlpha"
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
 * Clean 1dp bordered surface card with semantic elevation hierarchy.
 */
@Composable
fun VesperCard(
    modifier: Modifier = Modifier,
    cornerRadius: Dp = 14.dp,
    containerColor: Color = UnsetColor,
    accentTone: Color? = null,
    borderColor: Color = UnsetColor,
    elevated: Boolean = false,
    onClick: (() -> Unit)? = null,
    onLongClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit
) {
    val theme = BotHubThemeTokens
    val baseBg = when {
        containerColor != UnsetColor -> containerColor
        elevated -> theme.surfaceCardElevated
        else -> theme.surfaceCard
    }
    val shape = RoundedCornerShape(cornerRadius)
    val bgColor = remember(baseBg, accentTone, theme.isLight) {
        if (accentTone != null) {
            val weight = if (theme.isLight) 0.055f else 0.08f
            Color(
                red = (baseBg.red * (1f - weight) + accentTone.red * weight).coerceIn(0f, 1f),
                green = (baseBg.green * (1f - weight) + accentTone.green * weight).coerceIn(0f, 1f),
                blue = (baseBg.blue * (1f - weight) + accentTone.blue * weight).coerceIn(0f, 1f),
                alpha = baseBg.alpha
            )
        } else {
            baseBg
        }
    }
    val resolvedBorder = when {
        borderColor != UnsetColor -> borderColor
        accentTone != null -> accentTone.copy(alpha = if (theme.isLight) 0.30f else 0.36f)
        else -> theme.border
    }

    val interactiveMod = if (onClick != null) {
        Modifier.vesperPressable(
            pressedScale = 0.986f,
            onLongClick = onLongClick,
            onClick = onClick
        )
    } else {
        Modifier
    }

    val shadowMod = if (theme.isLight) {
        Modifier.shadow(
            elevation = if (elevated) 2.dp else 1.dp,
            shape = shape,
            ambientColor = Color(0x100F172A),
            spotColor = Color(0x180F172A)
        )
    } else {
        Modifier
    }

    Column(
        modifier = modifier
            .then(interactiveMod)
            .then(shadowMod)
            .clip(shape)
            .background(bgColor, shape)
            .border(1.dp, resolvedBorder, shape),
        content = content
    )
}

/**
 * Primary high-contrast CTA button. Enforces comfortable >= 48dp touch height by default.
 */
@Composable
fun VesperPrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    leadingIcon: ImageVector? = null,
    cornerRadius: Dp = 12.dp,
    minHeight: Dp = 48.dp,
    contentPadding: PaddingValues = PaddingValues(horizontal = 16.dp, vertical = 10.dp)
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(cornerRadius)
    val fillColor = if (enabled) theme.accent else theme.surfaceInput
    val fgColor = if (enabled) theme.onAccent else theme.textMuted
    val borderMod = if (enabled) Modifier else Modifier.border(1.dp, theme.border, shape)

    Box(
        modifier = modifier
            .sizeIn(minWidth = 48.dp, minHeight = minHeight)
            .vesperPressable(
                enabled = enabled,
                pressedScale = 0.974f,
                hapticOnTap = true,
                onClick = onClick
            )
            .clip(shape)
            .background(fillColor, shape)
            .then(borderMod)
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
 * Secondary outlined / tonal button with neutral, accent, or danger tone.
 */
@Composable
fun VesperSecondaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    tone: VesperButtonTone = VesperButtonTone.NEUTRAL,
    leadingIcon: ImageVector? = null,
    cornerRadius: Dp = 12.dp,
    minHeight: Dp = 48.dp,
    contentPadding: PaddingValues = PaddingValues(horizontal = 14.dp, vertical = 9.dp)
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(cornerRadius)
    val fillColor = when {
        !enabled -> theme.surfaceInput
        tone == VesperButtonTone.DANGER -> theme.danger.copy(alpha = if (theme.isLight) 0.09f else 0.15f)
        tone == VesperButtonTone.ACCENT -> theme.accentSoft
        else -> theme.surfaceControl
    }
    val borderColor = when {
        !enabled -> theme.divider
        tone == VesperButtonTone.DANGER -> theme.danger.copy(alpha = if (theme.isLight) 0.32f else 0.42f)
        tone == VesperButtonTone.ACCENT -> theme.accentMuted
        else -> theme.border
    }
    val fgColor = when {
        !enabled -> theme.textMuted
        tone == VesperButtonTone.DANGER -> theme.danger
        tone == VesperButtonTone.ACCENT -> theme.accentForeground
        else -> theme.textPrimary
    }

    Box(
        modifier = modifier
            .sizeIn(minWidth = 48.dp, minHeight = minHeight)
            .vesperPressable(
                enabled = enabled,
                pressedScale = 0.974f,
                onClick = onClick
            )
            .clip(shape)
            .background(fillColor, shape)
            .border(1.dp, borderColor, shape)
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
                    fontSize = 13.5.sp,
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
 * Icon button that guarantees a minimum 48x48dp touch target while displaying a crisp visual glyph container.
 */
@Composable
fun VesperIconButton(
    icon: ImageVector,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    accent: Boolean = false,
    danger: Boolean = false,
    filled: Boolean = true,
    visualSize: Dp = 36.dp,
    iconSize: Dp = 18.dp,
    cornerRadius: Dp = 999.dp
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(cornerRadius)
    val bgColor = when {
        !filled -> Color.Transparent
        danger -> theme.danger.copy(alpha = if (theme.isLight) 0.10f else 0.16f)
        accent -> theme.accentSoft
        else -> theme.surfaceControl
    }
    val borderColor = when {
        !filled -> Color.Transparent
        danger -> theme.danger.copy(alpha = 0.36f)
        accent -> theme.accentMuted
        else -> theme.border
    }
    val tint = when {
        !enabled -> theme.textMuted
        danger -> theme.danger
        accent -> theme.accentForeground
        else -> theme.textPrimary
    }

    Box(
        modifier = modifier
            .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
            .vesperPressable(
                enabled = enabled,
                pressedScale = 0.92f,
                hapticOnTap = true,
                onClick = onClick
            ),
        contentAlignment = Alignment.Center
    ) {
        Box(
            modifier = Modifier
                .size(visualSize)
                .clip(shape)
                .then(
                    if (filled) {
                        Modifier
                            .background(bgColor, shape)
                            .border(1.dp, borderColor, shape)
                    } else {
                        Modifier
                    }
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = contentDescription,
                tint = tint,
                modifier = Modifier.size(iconSize)
            )
        }
    }
}

/**
 * Sliding toggle switch wrapped in a guaranteed >= 48x48dp touch target.
 */
@Composable
fun VesperSwitch(
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
            stiffness = 600f
        ),
        label = "vesperSwitchThumb"
    )
    val trackColor by animateColorAsState(
        targetValue = if (checked) theme.accent else theme.surfaceInput,
        animationSpec = tween(150),
        label = "vesperSwitchTrack"
    )
    val borderColor by animateColorAsState(
        targetValue = if (checked) theme.accent else theme.borderStrong,
        animationSpec = tween(150),
        label = "vesperSwitchBorder"
    )
    val thumbColor = if (checked) theme.onAccent else Color(0xFFFFFFFF)

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
            .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
            .then(clickMod),
        contentAlignment = Alignment.Center
    ) {
        Box(
            modifier = Modifier
                .width(46.dp)
                .height(26.dp)
                .graphicsLayer { alpha = if (enabled) 1f else 0.45f }
                .clip(CircleShape)
                .background(trackColor, CircleShape)
                .border(1.dp, borderColor, CircleShape)
                .padding(2.dp),
            contentAlignment = Alignment.CenterStart
        ) {
            Box(
                modifier = Modifier
                    .offset(x = thumbOffset)
                    .size(22.dp)
                    .shadow(
                        elevation = 2.dp,
                        shape = CircleShape,
                        ambientColor = Color(0x2E000000),
                        spotColor = Color(0x3D000000)
                    )
                    .background(thumbColor, CircleShape)
            )
        }
    }
}

/**
 * Clean recessed search field with 48dp height and comfortable clear target.
 */
@Composable
fun VesperSearchField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(12.dp)
    val borderColor by animateColorAsState(
        targetValue = if (focused) theme.accent else theme.border,
        animationSpec = tween(140),
        label = "searchBorder"
    )

    Row(
        modifier = modifier
            .fillMaxWidth()
            .sizeIn(minHeight = 44.dp)
            .clip(shape)
            .background(theme.surfaceInput, shape)
            .border(if (focused) 1.25.dp else 1.dp, borderColor, shape)
            .padding(start = 12.dp, end = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        Icon(
            imageVector = Icons.Default.Search,
            contentDescription = null,
            tint = if (focused) theme.accent else theme.textMuted,
            modifier = Modifier.size(17.dp)
        )
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            singleLine = true,
            textStyle = TextStyle(
                color = theme.textPrimary,
                fontSize = 14.sp,
                platformStyle = NoFontPaddingStyle
            ),
            cursorBrush = SolidColor(theme.accent),
            modifier = Modifier
                .weight(1f)
                .padding(vertical = 10.dp)
                .onFocusChanged { focused = it.isFocused },
            decorationBox = { inner ->
                Box(contentAlignment = Alignment.CenterStart) {
                    if (value.isEmpty()) {
                        Text(
                            text = placeholder,
                            style = TextStyle(
                                fontSize = 13.5.sp,
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
                    .size(40.dp)
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        role = Role.Button
                    ) { onValueChange("") },
                contentAlignment = Alignment.Center
            ) {
                Box(
                    modifier = Modifier
                        .size(20.dp)
                        .clip(CircleShape)
                        .background(theme.textMuted.copy(alpha = 0.20f)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Default.Close,
                        contentDescription = "清空搜索",
                        tint = theme.textPrimary,
                        modifier = Modifier.size(12.dp)
                    )
                }
            }
        } else {
            Spacer(modifier = Modifier.width(8.dp))
        }
    }
}

/**
 * Crisp semantic form input field with focus-aware border and 48dp minimum height.
 */
@Composable
fun VesperTextField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier,
    label: String = "",
    singleLine: Boolean = true,
    maxLines: Int = if (singleLine) 1 else 4,
    visualTransformation: VisualTransformation = VisualTransformation.None,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default
) {
    val theme = BotHubThemeTokens
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(12.dp)
    val borderColor by animateColorAsState(
        targetValue = if (focused) theme.accent else theme.border,
        animationSpec = tween(150),
        label = "textFieldBorder"
    )

    Column(
        modifier = modifier,
        verticalArrangement = Arrangement.spacedBy(6.dp)
    ) {
        if (label.isNotEmpty()) {
            Text(
                text = label,
                style = TextStyle(
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                    color = if (focused) theme.accentForeground else theme.textSecondary,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.padding(start = 2.dp)
            )
        }
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .sizeIn(minHeight = 48.dp)
                .clip(shape)
                .background(theme.surfaceInput, shape)
                .border(if (focused) 1.3.dp else 1.dp, borderColor, shape),
            contentAlignment = Alignment.CenterStart
        ) {
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                singleLine = singleLine,
                maxLines = maxLines,
                visualTransformation = visualTransformation,
                keyboardOptions = keyboardOptions,
                textStyle = TextStyle(
                    color = theme.textPrimary,
                    fontSize = 14.sp,
                    lineHeight = 20.sp
                ),
                cursorBrush = SolidColor(theme.accent),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 14.dp, vertical = 12.dp)
                    .onFocusChanged { focused = it.isFocused },
                decorationBox = { inner ->
                    Box(contentAlignment = Alignment.CenterStart) {
                        if (value.isEmpty()) {
                            Text(
                                text = placeholder,
                                fontSize = 13.5.sp,
                                color = theme.textMuted,
                                maxLines = maxLines,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                        inner()
                    }
                }
            )
        }
    }
}

/**
 * Clean avatar wrapper hosting GrokBotAvatar (single) or GrokBotGroupCluster (group).
 */
@Composable
fun BotAvatar(
    bot: BotItem?,
    allBots: List<BotItem>,
    avatarStyleOverride: String? = null,
    state: String = bot?.state ?: "idle",
    containerSize: Dp = 48.dp,
    showPresenceDot: Boolean = false,
    interactive: Boolean = false,
    animateIdle: Boolean = false,
    seedKey: String = bot?.id ?: "vesper",
    onClick: (() -> Unit)? = null,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val isGroup = bot?.isGroup == true
    val clickMod = if (onClick != null) {
        Modifier.vesperPressable(pressedScale = 0.94f, onClick = onClick)
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
                    size = containerSize,
                    interactive = interactive,
                    animateIdle = animateIdle
                )
            } else {
                GrokBotAvatar(
                    avatarStyle = avatarStyleOverride ?: bot?.avatarStyle ?: "butler:gold",
                    state = state,
                    size = containerSize,
                    interactive = interactive,
                    animateIdle = animateIdle,
                    seedKey = seedKey
                )
            }
        }

        if (showPresenceDot && !isGroup) {
            val dotColor = when (state) {
                "working" -> theme.accent
                "thinking" -> theme.warning
                "blocked" -> theme.danger
                else -> theme.success
            }
            Box(
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .offset(x = 1.dp, y = 1.dp)
                    .size(12.dp)
                    .background(theme.bgCanvas, CircleShape)
                    .padding(2.dp)
                    .background(dotColor, CircleShape)
            )
        }
    }
}

/**
 * Crisp tabular unread count badge immune to OEM handwritten font vertical stretching.
 */
@Composable
fun VesperBadge(
    count: Int,
    containerColor: Color = Color(0xFFEF4444),
    contentColor: Color = Color.White,
    modifier: Modifier = Modifier
) {
    if (count <= 0) return
    Box(
        modifier = modifier
            .defaultMinSize(minWidth = 18.dp, minHeight = 18.dp)
            .height(18.dp)
            .background(containerColor, RoundedCornerShape(999.dp))
            .padding(horizontal = 5.dp),
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
 * Clean non-wrapping status or role tag.
 */
@Composable
fun VesperTag(
    text: String,
    toneColor: Color = UnsetColor,
    bgColor: Color = UnsetColor,
    borderColor: Color = UnsetColor,
    textColor: Color = UnsetColor,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val resolvedBg = when {
        bgColor != UnsetColor -> bgColor
        toneColor != UnsetColor -> toneColor.copy(alpha = if (theme.isLight) 0.11f else 0.16f)
        else -> theme.accentSoft
    }
    val resolvedBorder = when {
        borderColor != UnsetColor -> borderColor
        toneColor != UnsetColor -> toneColor.copy(alpha = if (theme.isLight) 0.26f else 0.34f)
        else -> theme.accentMuted
    }
    val resolvedText = when {
        textColor != UnsetColor -> textColor
        toneColor != UnsetColor -> toneColor
        else -> theme.accentForeground
    }
    val shape = RoundedCornerShape(6.dp)
    Box(
        modifier = modifier
            .clip(shape)
            .background(resolvedBg, shape)
            .border(0.75.dp, resolvedBorder, shape)
            .padding(horizontal = 6.5.dp, vertical = 2.5.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = text,
            style = TextStyle(
                fontSize = 10.5.sp,
                fontWeight = FontWeight.SemiBold,
                color = resolvedText,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false
        )
    }
}

/**
 * Filter pill chip with crisp active contrast and accessible touch target height.
 */
@Composable
fun VesperFilterChip(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
    leadingIcon: ImageVector? = null,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(999.dp)
    val bgColor = if (selected) theme.accentSoft else theme.surfaceCard
    val borderColor = if (selected) theme.accentMuted else theme.border
    val fgColor = if (selected) theme.accentForeground else theme.textSecondary

    Box(
        modifier = modifier
            .sizeIn(minHeight = 40.dp)
            .vesperPressable(pressedScale = 0.96f, hapticOnTap = true, onClick = onClick)
            .padding(vertical = 3.dp),
        contentAlignment = Alignment.Center
    ) {
        Row(
            modifier = Modifier
                .clip(shape)
                .background(bgColor, shape)
                .border(1.dp, borderColor, shape)
                .padding(horizontal = 13.dp, vertical = 7.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(5.dp)
        ) {
            if (leadingIcon != null) {
                Icon(
                    imageVector = leadingIcon,
                    contentDescription = null,
                    tint = fgColor,
                    modifier = Modifier.size(14.dp)
                )
            }
            Text(
                text = label,
                style = TextStyle(
                    fontSize = 12.5.sp,
                    fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                    color = fgColor,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false
            )
        }
    }
}

/**
 * Clean Linear/Raycast-inspired segmented tab control with 44-48dp comfortable touch height.
 */
@Composable
fun VesperSegmentedTabs(
    options: List<String>,
    selectedIndex: Int,
    onSelect: (Int) -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val outerShape = RoundedCornerShape(12.dp)
    val pillShape = RoundedCornerShape(9.dp)

    Row(
        modifier = modifier
            .fillMaxWidth()
            .clip(outerShape)
            .background(theme.surfaceInput, outerShape)
            .border(1.dp, theme.border, outerShape)
            .padding(3.dp),
        horizontalArrangement = Arrangement.spacedBy(3.dp)
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
                    .sizeIn(minHeight = 38.dp)
                    .vesperPressable(
                        pressedScale = 0.97f,
                        hapticOnTap = true,
                        onClick = { if (!selected) onSelect(idx) }
                    )
                    .clip(pillShape)
                    .then(
                        if (selected) {
                            Modifier
                                .background(theme.surfaceCard, pillShape)
                                .border(1.dp, theme.borderStrong, pillShape)
                        } else {
                            Modifier
                        }
                    )
                    .padding(horizontal = 6.dp, vertical = 7.dp),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = label,
                    style = TextStyle(
                        fontSize = 12.5.sp,
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

/**
 * Consistent section label header with optional count and trailing slot.
 */
@Composable
fun VesperSectionHeader(
    title: String,
    modifier: Modifier = Modifier,
    count: Int? = null,
    trailing: (@Composable () -> Unit)? = null
) {
    val theme = BotHubThemeTokens
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
            modifier = Modifier.padding(start = 2.dp)
        ) {
            Text(
                text = title,
                style = TextStyle(
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = theme.textMuted,
                    letterSpacing = 0.2.sp,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false
            )
            if (count != null) {
                Text(
                    text = count.toString(),
                    style = TextStyle(
                        fontSize = 11.sp,
                        fontFamily = FontFamily.Monospace,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textMuted,
                        platformStyle = NoFontPaddingStyle
                    ),
                    maxLines = 1,
                    softWrap = false
                )
            }
        }
        trailing?.invoke()
    }
}

/**
 * Clean centered empty state with vector icon, title, optional body and CTA slot. Zero emojis.
 */
@Composable
fun VesperEmptyState(
    icon: ImageVector,
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String = "",
    actions: (@Composable () -> Unit)? = null
) {
    val theme = BotHubThemeTokens
    VesperCard(
        cornerRadius = 16.dp,
        modifier = modifier.fillMaxWidth()
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp, vertical = 28.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(44.dp)
                    .clip(RoundedCornerShape(12.dp))
                    .background(theme.accentSoft)
                    .border(1.dp, theme.accentMuted, RoundedCornerShape(12.dp)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = theme.accentForeground,
                    modifier = Modifier.size(21.dp)
                )
            }
            Text(
                text = title,
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold,
                color = theme.textPrimary,
                textAlign = TextAlign.Center
            )
            if (subtitle.isNotBlank()) {
                Text(
                    text = subtitle,
                    fontSize = 12.5.sp,
                    lineHeight = 18.sp,
                    color = theme.textMuted,
                    textAlign = TextAlign.Center
                )
            }
            if (actions != null) {
                Spacer(modifier = Modifier.height(2.dp))
                actions()
            }
        }
    }
}

/**
 * Subtle 0.5dp hairline divider with optional horizontal inset.
 */
@Composable
fun VesperDivider(
    modifier: Modifier = Modifier,
    startIndent: Dp = 0.dp,
    endIndent: Dp = 0.dp
) {
    val theme = BotHubThemeTokens
    HorizontalDivider(
        thickness = 0.5.dp,
        color = theme.divider,
        modifier = modifier.padding(start = startIndent, end = endIndent)
    )
}

/**
 * Minimal bottom sheet drag indicator.
 */
@Composable
fun VesperSheetDragHandle(modifier: Modifier = Modifier) {
    val theme = BotHubThemeTokens
    Box(
        modifier = modifier
            .fillMaxWidth()
            .padding(top = 10.dp, bottom = 8.dp),
        contentAlignment = Alignment.Center
    ) {
        Box(
            modifier = Modifier
                .width(36.dp)
                .height(4.dp)
                .background(theme.textMuted.copy(alpha = 0.36f), RoundedCornerShape(999.dp))
        )
    }
}

data class VesperMenuAction(
    val label: String,
    val icon: ImageVector? = null,
    val iconTint: Color = UnsetColor,
    val danger: Boolean = false,
    val onClick: () -> Unit
)

/**
 * Clean floating popover menu with 48dp touch rows and crisp elevation.
 */
@Composable
fun VesperPopoverMenu(
    expanded: Boolean,
    onDismissRequest: () -> Unit,
    items: List<VesperMenuAction>,
    alignment: Alignment = Alignment.TopEnd,
    offsetX: Dp = 0.dp,
    offsetY: Dp = 44.dp
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
            enter = fadeIn(tween(120)) + scaleIn(
                initialScale = 0.94f,
                transformOrigin = TransformOrigin(0.9f, 0f),
                animationSpec = spring(dampingRatio = Spring.DampingRatioNoBouncy, stiffness = 680f)
            ),
            exit = fadeOut(tween(80)) + scaleOut(targetScale = 0.95f)
        ) {
            val shape = RoundedCornerShape(14.dp)
            Column(
                modifier = Modifier
                    .padding(horizontal = 8.dp, vertical = 4.dp)
                    .widthIn(min = 180.dp, max = 232.dp)
                    .shadow(
                        elevation = 10.dp,
                        shape = shape,
                        ambientColor = if (theme.isLight) Color(0x220F172A) else Color(0x99000000),
                        spotColor = if (theme.isLight) Color(0x2E0F172A) else Color(0xB8000000)
                    )
                    .clip(shape)
                    .background(theme.surfacePopover, shape)
                    .border(1.dp, theme.borderStrong, shape)
                    .padding(vertical = 4.dp)
            ) {
                items.forEachIndexed { idx, action ->
                    val effectiveTint = when {
                        action.danger -> theme.danger
                        action.iconTint != UnsetColor -> action.iconTint
                        else -> theme.accentForeground
                    }
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 48.dp)
                            .clickable {
                                onDismissRequest()
                                action.onClick()
                            }
                            .padding(horizontal = 14.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        if (action.icon != null) {
                            Box(
                                modifier = Modifier
                                    .size(28.dp)
                                    .clip(RoundedCornerShape(8.dp))
                                    .background(effectiveTint.copy(alpha = 0.13f)),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = action.icon,
                                    contentDescription = null,
                                    tint = effectiveTint,
                                    modifier = Modifier.size(16.dp)
                                )
                            }
                        }
                        Text(
                            text = action.label,
                            style = TextStyle(
                                fontSize = 14.sp,
                                fontWeight = FontWeight.Medium,
                                color = if (action.danger) theme.danger else theme.textPrimary,
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    if (idx < items.lastIndex) {
                        VesperDivider(
                            startIndent = if (action.icon != null) 54.dp else 14.dp,
                            endIndent = 14.dp
                        )
                    }
                }
            }
        }
    }
}

/**
 * Theme-unified floating pill Snackbar host with vector status cue.
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
        val statusColor = if (isError) theme.danger else theme.success
        val statusIcon = if (isError) Icons.Default.ErrorOutline else Icons.Default.CheckCircleOutline
        val shape = RoundedCornerShape(14.dp)

        Row(
            modifier = Modifier
                .padding(horizontal = 16.dp, vertical = 10.dp)
                .shadow(
                    elevation = 10.dp,
                    shape = shape,
                    ambientColor = if (theme.isLight) Color(0x220F172A) else Color(0x99000000),
                    spotColor = if (theme.isLight) Color(0x2E0F172A) else Color(0xB8000000)
                )
                .clip(shape)
                .background(theme.surfacePopover, shape)
                .border(1.dp, theme.borderStrong, shape)
                .clickable { data.dismiss() }
                .padding(horizontal = 14.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            Icon(
                imageVector = statusIcon,
                contentDescription = null,
                tint = statusColor,
                modifier = Modifier.size(17.dp)
            )
            Text(
                text = msg,
                style = TextStyle(
                    fontSize = 13.5.sp,
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
