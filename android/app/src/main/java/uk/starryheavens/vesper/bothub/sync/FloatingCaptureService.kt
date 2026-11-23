package uk.starryheavens.vesper.bothub.sync

import android.animation.Animator
import android.animation.AnimatorListenerAdapter
import android.animation.ValueAnimator
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.res.ColorStateList
import android.content.res.Configuration
import android.graphics.Color
import android.graphics.PixelFormat
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.provider.Settings
import android.util.DisplayMetrics
import android.util.TypedValue
import android.view.Gravity
import android.view.HapticFeedbackConstants
import android.view.MotionEvent
import android.view.View
import android.view.WindowInsets
import android.view.WindowManager
import android.view.animation.PathInterpolator
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.compose.ui.graphics.toArgb
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import uk.starryheavens.vesper.bothub.R
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeColors
import uk.starryheavens.vesper.bothub.ui.theme.resolveBotHubThemeColors
import kotlin.math.abs
import kotlin.math.hypot
import kotlin.math.roundToInt

class FloatingCaptureService : Service() {

    private var windowManager: WindowManager? = null
    private lateinit var prefs: BotHubPrefs

    // Primary docked handle overlay
    private var rootView: FrameLayout? = null
    private var pillView: FrameLayout? = null
    private var iconView: ImageView? = null
    private var statusDotView: View? = null
    private var grabBarView: View? = null
    private var layoutParams: WindowManager.LayoutParams? = null

    // Secondary floating quick-action menu overlay (long-press)
    private var menuView: LinearLayout? = null
    private var menuParams: WindowManager.LayoutParams? = null

    // Bottom magnetic dismiss target overlay (visible during drag)
    private var dismissContainer: FrameLayout? = null
    private var dismissPillView: LinearLayout? = null
    private var dismissIconView: ImageView? = null
    private var dismissLabelView: TextView? = null
    private var dismissParams: WindowManager.LayoutParams? = null
    private var inDismissZone: Boolean = false

    private var dockedRight: Boolean = true
    private var temporarilyHidden: Boolean = false
    private var isIdleTucked: Boolean = false
    private var isDraggingMode: Boolean = false

    private var snapAnimator: ValueAnimator? = null
    private val smoothDecel = PathInterpolator(0.20f, 1.0f, 0.32f, 1.0f)
    private val snappyOut = PathInterpolator(0.22f, 1.0f, 0.36f, 1.0f)

    @Volatile
    private var captureInProgress: Boolean = false

    private val idleTuckRunnable = Runnable {
        applyIdleTuck(tuck = true, animate = true)
    }

    private val captureWatchdogRunnable = Runnable {
        if (captureInProgress) {
            captureInProgress = false
            if (!temporarilyHidden) {
                rootView?.let { v ->
                    v.visibility = View.VISIBLE
                    pillView?.animate()?.alpha(ACTIVE_ALPHA)?.scaleX(1f)?.scaleY(1f)?.setDuration(150L)?.start()
                }
                scheduleIdleTuck()
            }
        }
    }

    override fun onCreate() {
        super.onCreate()
        activeInstance = this
        prefs = BotHubPrefs(applicationContext)
        dockedRight = prefs.floatingBallEdgeRight
        windowManager = getSystemService(Context.WINDOW_SERVICE) as? WindowManager
        if (!canDrawOverlays(this) || !prefs.floatingBallEnabled) {
            stopSelf()
            return
        }
        attachOverlayView()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (!canDrawOverlays(this) || !prefs.floatingBallEnabled) {
            stopSelf()
            return START_NOT_STICKY
        }
        when (intent?.action) {
            ACTION_SET_HIDDEN -> {
                applyHiddenState(intent.getBooleanExtra(EXTRA_HIDDEN, false))
            }
            ACTION_REFRESH_APPEARANCE -> {
                mainHandler.post { applyThemeStyling() }
            }
            else -> {
                if (rootView == null) {
                    attachOverlayView()
                } else {
                    applyHiddenState(temporarilyHidden)
                    applyThemeStyling()
                }
            }
        }
        return START_STICKY
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        mainHandler.post {
            closeQuickMenu(animate = false)
            hideDismissZone(animate = false)
            repositionForCurrentBounds()
            applyThemeStyling()
        }
    }

    override fun onDestroy() {
        if (activeInstance === this) {
            activeInstance = null
        }
        cancelAllPendingWork()
        closeQuickMenu(animate = false)
        hideDismissZone(animate = false)
        detachOverlayView()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun dp(value: Float): Int = (value * resources.displayMetrics.density + 0.5f).toInt()

    private fun dpF(value: Float): Float = value * resources.displayMetrics.density

    private fun currentTheme(): BotHubThemeColors {
        val nightMask = resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK
        val systemDark = nightMask == Configuration.UI_MODE_NIGHT_YES
        return resolveBotHubThemeColors(prefs.appThemeId, systemDark)
    }

    private fun withAlpha(argb: Int, alphaFraction: Float): Int {
        val alpha = (alphaFraction.coerceIn(0f, 1f) * 255f).roundToInt().coerceIn(0, 255)
        return (argb and 0x00FFFFFF) or (alpha shl 24)
    }

    private fun blendColor(baseArgb: Int, overlayArgb: Int, ratio: Float): Int {
        val t = ratio.coerceIn(0f, 1f)
        val r = (Color.red(baseArgb) * (1f - t) + Color.red(overlayArgb) * t).roundToInt().coerceIn(0, 255)
        val g = (Color.green(baseArgb) * (1f - t) + Color.green(overlayArgb) * t).roundToInt().coerceIn(0, 255)
        val b = (Color.blue(baseArgb) * (1f - t) + Color.blue(overlayArgb) * t).roundToInt().coerceIn(0, 255)
        val a = Color.alpha(baseArgb)
        return Color.argb(a, r, g, b)
    }

    private data class SafeScreenMetrics(
        val width: Int,
        val height: Int,
        val topInset: Int,
        val bottomInset: Int
    )

    private fun safeScreenMetrics(): SafeScreenMetrics {
        val wm = windowManager
        if (wm != null && Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            val metrics = wm.currentWindowMetrics
            val bounds = metrics.bounds
            val insets = metrics.windowInsets.getInsetsIgnoringVisibility(
                WindowInsets.Type.statusBars() or
                    WindowInsets.Type.navigationBars() or
                    WindowInsets.Type.displayCutout()
            )
            return SafeScreenMetrics(
                width = bounds.width(),
                height = bounds.height(),
                topInset = insets.top.coerceAtLeast(dp(36f)),
                bottomInset = insets.bottom.coerceAtLeast(dp(32f))
            )
        }
        val dm: DisplayMetrics = resources.displayMetrics
        return SafeScreenMetrics(
            width = dm.widthPixels,
            height = dm.heightPixels,
            topInset = dp(44f),
            bottomInset = dp(44f)
        )
    }

    private fun applyHiddenState(hidden: Boolean) {
        temporarilyHidden = hidden
        mainHandler.post {
            if (temporarilyHidden) {
                cancelIdleTuck()
                closeQuickMenu(animate = false)
                hideDismissZone(animate = false)
                rootView?.visibility = View.GONE
            } else {
                val rv = rootView
                if (rv == null) {
                    attachOverlayView()
                } else {
                    applyThemeStyling()
                    rv.visibility = View.VISIBLE
                    wakeFromIdle(animate = false)
                    scheduleIdleTuck()
                }
            }
        }
    }

    private fun cancelAllPendingWork() {
        mainHandler.removeCallbacks(idleTuckRunnable)
        mainHandler.removeCallbacks(captureWatchdogRunnable)
        snapAnimator?.cancel()
        snapAnimator = null
    }

    private fun cancelIdleTuck() {
        mainHandler.removeCallbacks(idleTuckRunnable)
    }

    private fun scheduleIdleTuck() {
        cancelIdleTuck()
        if (!temporarilyHidden && !isDraggingMode && menuView == null && !captureInProgress) {
            mainHandler.postDelayed(idleTuckRunnable, IDLE_TUCK_DELAY_MS)
        }
    }

    private fun wakeFromIdle(animate: Boolean) {
        cancelIdleTuck()
        isIdleTucked = false
        val pv = pillView ?: return
        pv.animate().cancel()
        if (animate) {
            pv.animate()
                .translationX(0f)
                .alpha(ACTIVE_ALPHA)
                .setDuration(140L)
                .setInterpolator(snappyOut)
                .start()
        } else {
            pv.translationX = 0f
            pv.alpha = ACTIVE_ALPHA
        }
    }

    private fun applyIdleTuck(tuck: Boolean, animate: Boolean) {
        if (isDraggingMode || menuView != null || temporarilyHidden || captureInProgress) return
        isIdleTucked = tuck
        val pv = pillView ?: return
        val targetTx = if (tuck) {
            if (dockedRight) dpF(IDLE_TUCK_OFFSET_DP) else -dpF(IDLE_TUCK_OFFSET_DP)
        } else {
            0f
        }
        val targetAlpha = if (tuck) IDLE_ALPHA else ACTIVE_ALPHA
        pv.animate().cancel()
        if (animate) {
            pv.animate()
                .translationX(targetTx)
                .alpha(targetAlpha)
                .setDuration(280L)
                .setInterpolator(smoothDecel)
                .start()
        } else {
            pv.translationX = targetTx
            pv.alpha = targetAlpha
        }
    }

    private fun buildHandleBackground(
        theme: BotHubThemeColors,
        floatingOrb: Boolean,
        dangerHighlight: Boolean = false
    ): GradientDrawable {
        val fillBase = theme.surfacePopover.toArgb()
        val fillColor = when {
            dangerHighlight -> withAlpha(blendColor(fillBase, theme.danger.toArgb(), 0.24f), 0.96f)
            floatingOrb -> withAlpha(blendColor(fillBase, theme.accent.toArgb(), 0.10f), 0.96f)
            else -> withAlpha(fillBase, if (theme.isLight) 0.95f else 0.92f)
        }
        val strokeColor = when {
            dangerHighlight -> theme.danger.toArgb()
            floatingOrb -> withAlpha(theme.accent.toArgb(), 0.72f)
            else -> theme.borderStrong.toArgb()
        }
        val strokeWidth = dp(if (floatingOrb || dangerHighlight) 1.35f else 1f).coerceAtLeast(1)

        return GradientDrawable().apply {
            shape = GradientDrawable.RECTANGLE
            if (floatingOrb) {
                cornerRadius = dpF(23f)
            } else {
                val r = dpF(20f)
                cornerRadii = if (dockedRight) {
                    floatArrayOf(r, r, 0f, 0f, 0f, 0f, r, r)
                } else {
                    floatArrayOf(0f, 0f, r, r, r, r, 0f, 0f)
                }
            }
            setColor(fillColor)
            setStroke(strokeWidth, strokeColor)
        }
    }

    private fun applyThemeStyling() {
        val theme = currentTheme()
        val pv = pillView ?: return
        val iv = iconView ?: return
        val dot = statusDotView ?: return
        val bar = grabBarView ?: return

        pv.background = buildHandleBackground(
            theme = theme,
            floatingOrb = isDraggingMode,
            dangerHighlight = inDismissZone
        )
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val shadowColor = if (theme.isLight) Color.parseColor("#330F172A") else Color.parseColor("#99000000")
            pv.outlineAmbientShadowColor = shadowColor
            pv.outlineSpotShadowColor = shadowColor
        }

        val iconColor = if (inDismissZone) theme.danger.toArgb() else theme.textPrimary.toArgb()
        iv.imageTintList = ColorStateList.valueOf(iconColor)

        val accessibilityReady = ScreenCaptureAccessibilityService.activeService != null
        val dotColor = if (accessibilityReady) theme.success.toArgb() else theme.accent.toArgb()
        dot.background = GradientDrawable().apply {
            shape = GradientDrawable.OVAL
            setColor(dotColor)
            setStroke(dp(1f).coerceAtLeast(1), theme.surfacePopover.toArgb())
        }

        bar.background = GradientDrawable().apply {
            shape = GradientDrawable.RECTANGLE
            cornerRadius = dpF(2f)
            setColor(withAlpha(theme.textMuted.toArgb(), 0.48f))
        }

        updateChildGravitiesForEdge()
    }

    private fun updateChildGravitiesForEdge() {
        val pv = pillView ?: return
        val iv = iconView ?: return
        val dot = statusDotView ?: return
        val bar = grabBarView ?: return

        val pvLp = pv.layoutParams as? FrameLayout.LayoutParams
        if (pvLp != null) {
            if (isDraggingMode) {
                pvLp.width = dp(ORB_SIZE_DP)
                pvLp.height = dp(ORB_SIZE_DP)
                pvLp.gravity = Gravity.CENTER
            } else {
                pvLp.width = dp(DOCK_PILL_W_DP)
                pvLp.height = dp(DOCK_PILL_H_DP)
                pvLp.gravity = Gravity.CENTER_VERTICAL or if (dockedRight) Gravity.END else Gravity.START
            }
            pv.layoutParams = pvLp
        }

        val ivLp = iv.layoutParams as? FrameLayout.LayoutParams
        if (ivLp != null) {
            ivLp.gravity = Gravity.CENTER
            if (isDraggingMode) {
                ivLp.marginStart = 0
                ivLp.marginEnd = 0
            } else if (dockedRight) {
                ivLp.marginStart = dp(4f)
                ivLp.marginEnd = dp(2f)
            } else {
                ivLp.marginStart = dp(2f)
                ivLp.marginEnd = dp(4f)
            }
            iv.layoutParams = ivLp
        }

        val dotLp = dot.layoutParams as? FrameLayout.LayoutParams
        if (dotLp != null) {
            if (isDraggingMode) {
                dotLp.gravity = Gravity.TOP or Gravity.END
                dotLp.topMargin = dp(8f)
                dotLp.marginEnd = dp(8f)
                dotLp.marginStart = 0
            } else if (dockedRight) {
                dotLp.gravity = Gravity.TOP or Gravity.START
                dotLp.topMargin = dp(8f)
                dotLp.marginStart = dp(8f)
                dotLp.marginEnd = 0
            } else {
                dotLp.gravity = Gravity.TOP or Gravity.END
                dotLp.topMargin = dp(8f)
                dotLp.marginEnd = dp(8f)
                dotLp.marginStart = 0
            }
            dot.layoutParams = dotLp
        }

        val barLp = bar.layoutParams as? FrameLayout.LayoutParams
        if (barLp != null) {
            barLp.gravity = Gravity.CENTER_VERTICAL or if (dockedRight) Gravity.START else Gravity.END
            barLp.marginStart = if (dockedRight) dp(4f) else 0
            barLp.marginEnd = if (dockedRight) 0 else dp(4f)
            bar.layoutParams = barLp
            bar.visibility = if (isDraggingMode) View.GONE else View.VISIBLE
        }
    }

    private fun attachOverlayView() {
        val wm = windowManager ?: return
        if (rootView != null) return

        val metrics = safeScreenMetrics()
        val windowW = dp(HANDLE_WINDOW_W_DP)
        val windowH = dp(HANDLE_WINDOW_H_DP)
        val minY = metrics.topInset + dp(12f)
        val maxY = (metrics.height - metrics.bottomInset - windowH - dp(12f)).coerceAtLeast(minY)
        val initialY = (metrics.height * prefs.floatingBallYFraction).toInt().coerceIn(minY, maxY)

        val outerContainer = FrameLayout(this).apply {
            clipChildren = false
            clipToPadding = false
            contentDescription = "BotHub 悬浮快捷入口"
        }

        val pill = FrameLayout(this).apply {
            elevation = dpF(8f)
            alpha = ACTIVE_ALPHA
            clipChildren = false
            clipToPadding = false
        }

        val grabBar = View(this)
        pill.addView(
            grabBar,
            FrameLayout.LayoutParams(dp(2.5f), dp(16f), Gravity.CENTER_VERTICAL or Gravity.START)
        )

        val icon = ImageView(this).apply {
            setImageResource(R.drawable.ic_notification)
            scaleType = ImageView.ScaleType.FIT_CENTER
        }
        val iconSize = dp(20f)
        pill.addView(icon, FrameLayout.LayoutParams(iconSize, iconSize, Gravity.CENTER))

        val statusDot = View(this)
        val dotSize = dp(6.5f)
        pill.addView(statusDot, FrameLayout.LayoutParams(dotSize, dotSize, Gravity.TOP or Gravity.START))

        outerContainer.addView(
            pill,
            FrameLayout.LayoutParams(
                dp(DOCK_PILL_W_DP),
                dp(DOCK_PILL_H_DP),
                Gravity.CENTER_VERTICAL or if (dockedRight) Gravity.END else Gravity.START
            )
        )

        val params = WindowManager.LayoutParams(
            windowW,
            windowH,
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN,
            PixelFormat.TRANSLUCENT
        ).apply {
            gravity = Gravity.TOP or Gravity.START
            x = if (dockedRight) (metrics.width - windowW).coerceAtLeast(0) else 0
            y = initialY
        }

        rootView = outerContainer
        pillView = pill
        iconView = icon
        statusDotView = statusDot
        grabBarView = grabBar
        layoutParams = params

        applyThemeStyling()
        setupHandleTouchGestures(outerContainer, wm)

        outerContainer.visibility = if (temporarilyHidden) View.GONE else View.VISIBLE
        val added = runCatching {
            wm.addView(outerContainer, params)
            true
        }.getOrDefault(false)

        if (added) {
            if (!temporarilyHidden) {
                scheduleIdleTuck()
            }
        } else {
            rootView = null
            pillView = null
            iconView = null
            statusDotView = null
            grabBarView = null
            layoutParams = null
            stopSelf()
        }
    }

    private fun setupHandleTouchGestures(container: FrameLayout, wm: WindowManager) {
        val touchSlop = dpF(10f)
        val windowW = dp(HANDLE_WINDOW_W_DP)
        val windowH = dp(HANDLE_WINDOW_H_DP)

        var downRawX = 0f
        var downRawY = 0f
        var startX = 0
        var startY = 0
        var dragging = false
        var longPressTriggered = false

        val longPressRunnable = Runnable {
            if (!dragging && !captureInProgress) {
                longPressTriggered = true
                container.performHapticFeedback(HapticFeedbackConstants.LONG_PRESS)
                pillView?.animate()?.scaleX(1f)?.scaleY(1f)?.setDuration(120L)?.start()
                if (menuView != null) {
                    closeQuickMenu(animate = true)
                } else {
                    showQuickMenu()
                }
            }
        }

        container.setOnTouchListener { view, event ->
            val lp = layoutParams ?: return@setOnTouchListener false
            when (event.actionMasked) {
                MotionEvent.ACTION_DOWN -> {
                    snapAnimator?.cancel()
                    downRawX = event.rawX
                    downRawY = event.rawY
                    startX = lp.x
                    startY = lp.y
                    dragging = false
                    longPressTriggered = false
                    inDismissZone = false

                    wakeFromIdle(animate = true)
                    pillView?.animate()
                        ?.scaleX(0.93f)
                        ?.scaleY(0.93f)
                        ?.setDuration(100L)
                        ?.setInterpolator(snappyOut)
                        ?.start()

                    mainHandler.removeCallbacks(longPressRunnable)
                    mainHandler.postDelayed(longPressRunnable, LONG_PRESS_TIMEOUT_MS)
                    true
                }

                MotionEvent.ACTION_MOVE -> {
                    val dx = event.rawX - downRawX
                    val dy = event.rawY - downRawY
                    if (!dragging && hypot(dx.toDouble(), dy.toDouble()) > touchSlop) {
                        dragging = true
                        mainHandler.removeCallbacks(longPressRunnable)
                        if (menuView != null) {
                            closeQuickMenu(animate = true)
                        }
                        enterDraggingVisualState()
                        showDismissZone()
                    }
                    if (dragging) {
                        val metrics = safeScreenMetrics()
                        val minY = metrics.topInset
                        val maxY = (metrics.height - metrics.bottomInset - windowH).coerceAtLeast(minY)
                        lp.x = (startX + dx.toInt()).coerceIn(0, (metrics.width - windowW).coerceAtLeast(0))
                        lp.y = (startY + dy.toInt()).coerceIn(minY, maxY)
                        runCatching { wm.updateViewLayout(view, lp) }

                        val ballCenterX = lp.x + windowW / 2f
                        val ballCenterY = lp.y + windowH / 2f
                        val targetCenterX = metrics.width / 2f
                        val targetCenterY = (metrics.height - metrics.bottomInset - dp(48f)).toFloat()
                        val distToDismiss = hypot(
                            (ballCenterX - targetCenterX).toDouble(),
                            (ballCenterY - targetCenterY).toDouble()
                        )
                        val entered = distToDismiss <= dpF(88f) ||
                            (ballCenterY >= metrics.height - metrics.bottomInset - dp(72f) &&
                                abs(ballCenterX - targetCenterX) <= dpF(112f))

                        if (entered != inDismissZone) {
                            inDismissZone = entered
                            view.performHapticFeedback(HapticFeedbackConstants.CLOCK_TICK)
                            updateDismissZoneHighlight(entered)
                            updateDraggingOrbHighlight(entered)
                        }
                    }
                    true
                }

                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                    mainHandler.removeCallbacks(longPressRunnable)
                    if (longPressTriggered) {
                        true
                    } else if (dragging) {
                        val shouldDismiss = inDismissZone && event.actionMasked == MotionEvent.ACTION_UP
                        inDismissZone = false
                        hideDismissZone(animate = true)
                        if (shouldDismiss) {
                            view.performHapticFeedback(HapticFeedbackConstants.LONG_PRESS)
                            dismissByUser()
                        } else {
                            animateSnapToNearestEdge(view, lp, wm)
                        }
                        true
                    } else {
                        pillView?.animate()?.scaleX(1f)?.scaleY(1f)?.setDuration(110L)?.start()
                        if (event.actionMasked == MotionEvent.ACTION_UP) {
                            view.performClick()
                            if (menuView != null) {
                                closeQuickMenu(animate = true)
                                scheduleIdleTuck()
                            } else {
                                view.performHapticFeedback(HapticFeedbackConstants.CONTEXT_CLICK)
                                triggerFloatingCapture(forceClipboardOnly = false)
                            }
                        } else {
                            scheduleIdleTuck()
                        }
                        true
                    }
                }

                else -> false
            }
        }
    }

    private fun enterDraggingVisualState() {
        isDraggingMode = true
        val pv = pillView ?: return
        applyThemeStyling()
        pv.animate().cancel()
        pv.translationX = 0f
        pv.alpha = 1f
        pv.elevation = dpF(12f)
        pv.animate()
            .scaleX(1.06f)
            .scaleY(1.06f)
            .setDuration(140L)
            .setInterpolator(snappyOut)
            .start()
    }

    private fun updateDraggingOrbHighlight(danger: Boolean) {
        val pv = pillView ?: return
        applyThemeStyling()
        val targetScale = if (danger) 0.84f else 1.06f
        pv.animate()
            .scaleX(targetScale)
            .scaleY(targetScale)
            .setDuration(120L)
            .setInterpolator(snappyOut)
            .start()
    }

    private fun animateSnapToNearestEdge(
        view: View,
        lp: WindowManager.LayoutParams,
        wm: WindowManager
    ) {
        val metrics = safeScreenMetrics()
        val windowW = dp(HANDLE_WINDOW_W_DP)
        val windowH = dp(HANDLE_WINDOW_H_DP)
        val centerX = lp.x + windowW / 2
        val targetRight = centerX >= metrics.width / 2
        val targetX = if (targetRight) (metrics.width - windowW).coerceAtLeast(0) else 0
        val minY = metrics.topInset + dp(12f)
        val maxY = (metrics.height - metrics.bottomInset - windowH - dp(12f)).coerceAtLeast(minY)
        val clampedY = lp.y.coerceIn(minY, maxY)

        val startX = lp.x
        val startY = lp.y
        snapAnimator?.cancel()
        snapAnimator = ValueAnimator.ofFloat(0f, 1f).apply {
            duration = 260L
            interpolator = smoothDecel
            addUpdateListener { anim ->
                val f = anim.animatedValue as Float
                lp.x = (startX + (targetX - startX) * f).roundToInt()
                lp.y = (startY + (clampedY - startY) * f).roundToInt()
                runCatching { wm.updateViewLayout(view, lp) }
            }
            addListener(object : AnimatorListenerAdapter() {
                override fun onAnimationEnd(animation: Animator) {
                    dockedRight = targetRight
                    isDraggingMode = false
                    inDismissZone = false
                    applyThemeStyling()
                    pillView?.let { pv ->
                        pv.elevation = dpF(8f)
                        pv.scaleX = 0.96f
                        pv.scaleY = 0.96f
                        pv.animate()
                            .scaleX(1f)
                            .scaleY(1f)
                            .alpha(ACTIVE_ALPHA)
                            .setDuration(140L)
                            .setInterpolator(snappyOut)
                            .start()
                    }
                    view.performHapticFeedback(HapticFeedbackConstants.CLOCK_TICK)
                    prefs.floatingBallEdgeRight = dockedRight
                    if (metrics.height > 0) {
                        prefs.floatingBallYFraction = clampedY.toFloat() / metrics.height.toFloat()
                    }
                    scheduleIdleTuck()
                }
            })
            start()
        }
    }

    private fun repositionForCurrentBounds() {
        val wm = windowManager ?: return
        val rv = rootView ?: return
        val lp = layoutParams ?: return
        val metrics = safeScreenMetrics()
        val windowW = dp(HANDLE_WINDOW_W_DP)
        val windowH = dp(HANDLE_WINDOW_H_DP)
        val minY = metrics.topInset + dp(12f)
        val maxY = (metrics.height - metrics.bottomInset - windowH - dp(12f)).coerceAtLeast(minY)
        lp.x = if (dockedRight) (metrics.width - windowW).coerceAtLeast(0) else 0
        lp.y = (metrics.height * prefs.floatingBallYFraction).toInt().coerceIn(minY, maxY)
        runCatching { wm.updateViewLayout(rv, lp) }
        if (isIdleTucked && !isDraggingMode) {
            applyIdleTuck(tuck = true, animate = false)
        }
    }

    private fun showDismissZone() {
        val wm = windowManager ?: return
        if (dismissContainer != null) return

        val theme = currentTheme()
        val metrics = safeScreenMetrics()

        val outer = FrameLayout(this).apply {
            clipChildren = false
            clipToPadding = false
        }

        val pill = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER
            setPadding(dp(18f), dp(12f), dp(18f), dp(12f))
            elevation = dpF(10f)
            alpha = 0f
            translationY = dpF(36f)
        }

        val icon = ImageView(this).apply {
            setImageResource(R.drawable.ic_float_close)
            scaleType = ImageView.ScaleType.FIT_CENTER
        }
        val iconLp = LinearLayout.LayoutParams(dp(16f), dp(16f)).apply {
            marginEnd = dp(8f)
        }
        pill.addView(icon, iconLp)

        val label = TextView(this).apply {
            text = "拖到此处关闭"
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 13f)
            setTypeface(Typeface.DEFAULT, Typeface.BOLD)
            includeFontPadding = false
        }
        pill.addView(label)

        outer.addView(
            pill,
            FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.WRAP_CONTENT,
                dp(48f),
                Gravity.CENTER
            ).apply {
                setMargins(dp(16f), dp(10f), dp(16f), dp(10f))
            }
        )

        val params = WindowManager.LayoutParams(
            WindowManager.LayoutParams.WRAP_CONTENT,
            WindowManager.LayoutParams.WRAP_CONTENT,
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE or
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN,
            PixelFormat.TRANSLUCENT
        ).apply {
            gravity = Gravity.BOTTOM or Gravity.CENTER_HORIZONTAL
            y = metrics.bottomInset + dp(18f)
        }

        dismissContainer = outer
        dismissPillView = pill
        dismissIconView = icon
        dismissLabelView = label
        dismissParams = params

        applyDismissPillStyle(theme, activeDanger = false)

        val added = runCatching {
            wm.addView(outer, params)
            true
        }.getOrDefault(false)

        if (added) {
            pill.animate()
                .alpha(1f)
                .translationY(0f)
                .setDuration(190L)
                .setInterpolator(snappyOut)
                .start()
        } else {
            dismissContainer = null
            dismissPillView = null
            dismissIconView = null
            dismissLabelView = null
            dismissParams = null
        }
    }

    private fun applyDismissPillStyle(theme: BotHubThemeColors, activeDanger: Boolean) {
        val pill = dismissPillView ?: return
        val icon = dismissIconView ?: return
        val label = dismissLabelView ?: return

        val fillBase = theme.surfacePopover.toArgb()
        val bgColor = if (activeDanger) {
            withAlpha(blendColor(fillBase, theme.danger.toArgb(), 0.24f), 0.98f)
        } else {
            withAlpha(fillBase, 0.94f)
        }
        val borderColor = if (activeDanger) theme.danger.toArgb() else theme.borderStrong.toArgb()
        val fgColor = if (activeDanger) theme.danger.toArgb() else theme.textSecondary.toArgb()

        pill.background = GradientDrawable().apply {
            shape = GradientDrawable.RECTANGLE
            cornerRadius = dpF(24f)
            setColor(bgColor)
            setStroke(dp(if (activeDanger) 1.4f else 1f).coerceAtLeast(1), borderColor)
        }
        icon.imageTintList = ColorStateList.valueOf(fgColor)
        label.setTextColor(fgColor)
        label.text = if (activeDanger) "松手关闭悬浮球" else "拖到此处关闭"
    }

    private fun updateDismissZoneHighlight(activeDanger: Boolean) {
        val theme = currentTheme()
        applyDismissPillStyle(theme, activeDanger)
        dismissPillView?.animate()
            ?.scaleX(if (activeDanger) 1.06f else 1f)
            ?.scaleY(if (activeDanger) 1.06f else 1f)
            ?.setDuration(130L)
            ?.setInterpolator(snappyOut)
            ?.start()
    }

    private fun hideDismissZone(animate: Boolean) {
        val wm = windowManager
        val container = dismissContainer ?: return
        val pill = dismissPillView
        dismissContainer = null
        dismissPillView = null
        dismissIconView = null
        dismissLabelView = null
        dismissParams = null

        if (animate && pill != null) {
            pill.animate()
                .alpha(0f)
                .translationY(dpF(32f))
                .setDuration(140L)
                .setInterpolator(smoothDecel)
                .setListener(object : AnimatorListenerAdapter() {
                    override fun onAnimationEnd(animation: Animator) {
                        runCatching { wm?.removeViewImmediate(container) }
                    }
                })
                .start()
        } else {
            runCatching { wm?.removeViewImmediate(container) }
        }
    }

    private fun showQuickMenu() {
        val wm = windowManager ?: return
        val lp = layoutParams ?: return
        closeQuickMenu(animate = false)
        cancelIdleTuck()
        wakeFromIdle(animate = true)

        val theme = currentTheme()
        val metrics = safeScreenMetrics()
        val accessibilityReady = ScreenCaptureAccessibilityService.activeService != null

        val menuCard = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(0, dp(6f), 0, dp(6f))
            elevation = dpF(12f)
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dpF(16f)
                setColor(withAlpha(theme.surfacePopover.toArgb(), 0.97f))
                setStroke(dp(1f).coerceAtLeast(1), theme.borderStrong.toArgb())
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                val shadowColor = if (theme.isLight) Color.parseColor("#2E0F172A") else Color.parseColor("#AD000000")
                outlineAmbientShadowColor = shadowColor
                outlineSpotShadowColor = shadowColor
            }
        }

        data class QuickActionSpec(
            val iconRes: Int,
            val label: String,
            val tintArgb: Int,
            val danger: Boolean = false,
            val onClick: () -> Unit
        )

        val actions = listOf(
            QuickActionSpec(
                iconRes = R.drawable.ic_float_scan,
                label = if (accessibilityReady) "识屏收藏" else "快捷收藏",
                tintArgb = theme.accentForeground.toArgb(),
                onClick = { triggerFloatingCapture(forceClipboardOnly = false) }
            ),
            QuickActionSpec(
                iconRes = R.drawable.ic_float_clipboard,
                label = "读取剪贴板",
                tintArgb = theme.info.toArgb(),
                onClick = { triggerFloatingCapture(forceClipboardOnly = true) }
            ),
            QuickActionSpec(
                iconRes = R.drawable.ic_float_chat,
                label = "打开 BotHub",
                tintArgb = theme.success.toArgb(),
                onClick = {
                    runCatching {
                        startActivity(
                            AppIntents.createMainActivityIntent(
                                context = applicationContext,
                                openTab = BotHubTab.AGENT
                            )
                        )
                    }
                }
            ),
            QuickActionSpec(
                iconRes = R.drawable.ic_float_close,
                label = "关闭悬浮球",
                tintArgb = theme.danger.toArgb(),
                danger = true,
                onClick = { dismissByUser() }
            )
        )

        actions.forEachIndexed { idx, spec ->
            val row = buildQuickMenuRow(
                theme = theme,
                iconRes = spec.iconRes,
                label = spec.label,
                tintArgb = spec.tintArgb,
                danger = spec.danger
            ) {
                closeQuickMenu(animate = false)
                spec.onClick()
            }
            menuCard.addView(
                row,
                LinearLayout.LayoutParams(dp(172f), LinearLayout.LayoutParams.WRAP_CONTENT)
            )
            if (idx < actions.lastIndex) {
                val divider = View(this).apply {
                    setBackgroundColor(theme.divider.toArgb())
                }
                val divLp = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    dp(0.6f).coerceAtLeast(1)
                ).apply {
                    marginStart = dp(46f)
                    marginEnd = dp(12f)
                }
                menuCard.addView(divider, divLp)
            }
        }

        menuCard.measure(
            View.MeasureSpec.makeMeasureSpec(dp(172f), View.MeasureSpec.EXACTLY),
            View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED)
        )
        val menuW = menuCard.measuredWidth.coerceAtLeast(dp(172f))
        val menuH = menuCard.measuredHeight.coerceAtLeast(dp(200f))

        val handleW = dp(HANDLE_WINDOW_W_DP)
        val handleH = dp(HANDLE_WINDOW_H_DP)
        val gap = dp(6f)
        val menuX = if (dockedRight) {
            (metrics.width - handleW - gap - menuW).coerceAtLeast(dp(12f))
        } else {
            (handleW + gap).coerceAtMost((metrics.width - menuW - dp(12f)).coerceAtLeast(dp(12f)))
        }
        val desiredY = lp.y + handleH / 2 - menuH / 2
        val minY = metrics.topInset + dp(12f)
        val maxY = (metrics.height - metrics.bottomInset - menuH - dp(12f)).coerceAtLeast(minY)
        val menuY = desiredY.coerceIn(minY, maxY)

        val params = WindowManager.LayoutParams(
            WindowManager.LayoutParams.WRAP_CONTENT,
            WindowManager.LayoutParams.WRAP_CONTENT,
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_WATCH_OUTSIDE_TOUCH or
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN,
            PixelFormat.TRANSLUCENT
        ).apply {
            gravity = Gravity.TOP or Gravity.START
            x = menuX
            y = menuY
        }

        menuCard.setOnTouchListener { _, event ->
            if (event.actionMasked == MotionEvent.ACTION_OUTSIDE) {
                closeQuickMenu(animate = true)
                scheduleIdleTuck()
                true
            } else {
                false
            }
        }

        menuCard.alpha = 0f
        menuCard.scaleX = 0.88f
        menuCard.scaleY = 0.88f
        menuCard.pivotX = if (dockedRight) menuW.toFloat() else 0f
        menuCard.pivotY = (menuH / 2f)

        val added = runCatching {
            wm.addView(menuCard, params)
            true
        }.getOrDefault(false)

        if (added) {
            menuView = menuCard
            menuParams = params
            menuCard.animate()
                .alpha(1f)
                .scaleX(1f)
                .scaleY(1f)
                .setDuration(165L)
                .setInterpolator(snappyOut)
                .start()
        }
    }

    private fun buildQuickMenuRow(
        theme: BotHubThemeColors,
        iconRes: Int,
        label: String,
        tintArgb: Int,
        danger: Boolean,
        onClick: () -> Unit
    ): LinearLayout {
        val row = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            minimumHeight = dp(48f)
            setPadding(dp(12f), dp(8f), dp(14f), dp(8f))
            isClickable = true
        }

        val badge = FrameLayout(this).apply {
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dpF(8f)
                setColor(withAlpha(tintArgb, 0.14f))
            }
        }
        val icon = ImageView(this).apply {
            setImageResource(iconRes)
            imageTintList = ColorStateList.valueOf(tintArgb)
            scaleType = ImageView.ScaleType.FIT_CENTER
        }
        badge.addView(icon, FrameLayout.LayoutParams(dp(16f), dp(16f), Gravity.CENTER))
        row.addView(
            badge,
            LinearLayout.LayoutParams(dp(28f), dp(28f)).apply {
                marginEnd = dp(10f)
            }
        )

        val textView = TextView(this).apply {
            text = label
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 13.5f)
            setTypeface(Typeface.DEFAULT, Typeface.BOLD)
            setTextColor(if (danger) theme.danger.toArgb() else theme.textPrimary.toArgb())
            includeFontPadding = false
            maxLines = 1
        }
        row.addView(textView)

        row.setOnTouchListener { v, event ->
            when (event.actionMasked) {
                MotionEvent.ACTION_DOWN -> {
                    v.setBackgroundColor(withAlpha(tintArgb, 0.10f))
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                    v.setBackgroundColor(Color.TRANSPARENT)
                }
            }
            false
        }
        row.setOnClickListener {
            it.performHapticFeedback(HapticFeedbackConstants.CONTEXT_CLICK)
            onClick()
        }
        return row
    }

    private fun closeQuickMenu(animate: Boolean) {
        val wm = windowManager
        val mv = menuView ?: return
        menuView = null
        menuParams = null

        if (animate) {
            mv.animate()
                .alpha(0f)
                .scaleX(0.90f)
                .scaleY(0.90f)
                .setDuration(110L)
                .setInterpolator(smoothDecel)
                .setListener(object : AnimatorListenerAdapter() {
                    override fun onAnimationEnd(animation: Animator) {
                        runCatching { wm?.removeViewImmediate(mv) }
                    }
                })
                .start()
        } else {
            runCatching { wm?.removeViewImmediate(mv) }
        }
    }

    private fun dismissByUser() {
        cancelAllPendingWork()
        closeQuickMenu(animate = false)
        hideDismissZone(animate = true)
        prefs.floatingBallEnabled = false
        _floatingBallDismissedAtMs.value = System.currentTimeMillis()

        val pv = pillView
        if (pv != null) {
            pv.animate()
                .scaleX(0.4f)
                .scaleY(0.4f)
                .alpha(0f)
                .setDuration(140L)
                .setInterpolator(smoothDecel)
                .setListener(object : AnimatorListenerAdapter() {
                    override fun onAnimationEnd(animation: Animator) {
                        Toast.makeText(
                            applicationContext,
                            "悬浮球已关闭，可在设置中随时重新开启",
                            Toast.LENGTH_SHORT
                        ).show()
                        stopSelf()
                    }
                })
                .start()
        } else {
            Toast.makeText(
                applicationContext,
                "悬浮球已关闭，可在设置中随时重新开启",
                Toast.LENGTH_SHORT
            ).show()
            stopSelf()
        }
    }

    private fun triggerFloatingCapture(forceClipboardOnly: Boolean) {
        if (captureInProgress) return
        closeQuickMenu(animate = false)
        cancelIdleTuck()

        val accessibilitySvc = if (forceClipboardOnly) null else ScreenCaptureAccessibilityService.activeService
        if (accessibilitySvc == null) {
            runCatching {
                startActivity(AppIntents.createClipboardCaptureIntent(applicationContext))
            }
            scheduleIdleTuck()
            return
        }

        captureInProgress = true
        mainHandler.removeCallbacks(captureWatchdogRunnable)
        mainHandler.postDelayed(captureWatchdogRunnable, CAPTURE_WATCHDOG_MS)

        // Hide cleanly right before taking the screenshot so the handle never appears in captures
        rootView?.visibility = View.GONE
        mainHandler.postDelayed({
            accessibilitySvc.captureCurrentScreen { snapshot ->
                mainHandler.removeCallbacks(captureWatchdogRunnable)
                captureInProgress = false
                ScreenCaptureHolder.setPending(snapshot)
                val launched = runCatching {
                    startActivity(AppIntents.createScreenCaptureIntent(applicationContext))
                    true
                }.getOrDefault(false)
                if (!launched && !temporarilyHidden) {
                    rootView?.visibility = View.VISIBLE
                    wakeFromIdle(animate = false)
                    scheduleIdleTuck()
                }
            }
        }, 90L)
    }

    private fun detachOverlayView() {
        val view = rootView ?: return
        rootView = null
        pillView = null
        iconView = null
        statusDotView = null
        grabBarView = null
        layoutParams = null
        runCatching { windowManager?.removeViewImmediate(view) }
    }

    companion object {
        private const val ACTION_SET_HIDDEN = "uk.starryheavens.vesper.bothub.FLOATING_SET_HIDDEN"
        private const val ACTION_REFRESH_APPEARANCE = "uk.starryheavens.vesper.bothub.FLOATING_REFRESH_APPEARANCE"
        private const val EXTRA_HIDDEN = "extra_hidden"

        private const val HANDLE_WINDOW_W_DP = 52f
        private const val HANDLE_WINDOW_H_DP = 60f
        private const val DOCK_PILL_W_DP = 40f
        private const val DOCK_PILL_H_DP = 50f
        private const val ORB_SIZE_DP = 46f
        private const val IDLE_TUCK_OFFSET_DP = 15f

        private const val ACTIVE_ALPHA = 0.96f
        private const val IDLE_ALPHA = 0.54f
        private const val IDLE_TUCK_DELAY_MS = 3200L
        private const val LONG_PRESS_TIMEOUT_MS = 360L
        private const val CAPTURE_WATCHDOG_MS = 5000L

        private val mainHandler = Handler(Looper.getMainLooper())

        @Volatile
        private var activeInstance: FloatingCaptureService? = null

        private val _floatingBallDismissedAtMs = MutableStateFlow(0L)
        val floatingBallDismissedAtMs: StateFlow<Long> = _floatingBallDismissedAtMs.asStateFlow()

        fun canDrawOverlays(context: Context): Boolean {
            return runCatching { Settings.canDrawOverlays(context.applicationContext) }.getOrDefault(false)
        }

        fun openOverlayPermissionSettings(context: Context) {
            val intent = Intent(
                Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                Uri.parse("package:${context.packageName}")
            ).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            runCatching { context.startActivity(intent) }
        }

        fun syncWithPrefs(context: Context, prefs: BotHubPrefs, appInForeground: Boolean = false) {
            val appContext = context.applicationContext
            if (prefs.floatingBallEnabled && canDrawOverlays(appContext)) {
                val running = activeInstance
                if (running != null) {
                    running.applyHiddenState(appInForeground)
                    return
                }
                val intent = Intent(appContext, FloatingCaptureService::class.java).apply {
                    action = ACTION_SET_HIDDEN
                    putExtra(EXTRA_HIDDEN, appInForeground)
                }
                runCatching { appContext.startService(intent) }
            } else {
                runCatching { appContext.stopService(Intent(appContext, FloatingCaptureService::class.java)) }
            }
        }

        fun setTemporarilyHidden(context: Context, hidden: Boolean) {
            val appContext = context.applicationContext
            val prefs = BotHubPrefs(appContext)
            if (!prefs.floatingBallEnabled || !canDrawOverlays(appContext)) return
            val running = activeInstance
            if (running != null) {
                running.applyHiddenState(hidden)
                return
            }
            val intent = Intent(appContext, FloatingCaptureService::class.java).apply {
                action = ACTION_SET_HIDDEN
                putExtra(EXTRA_HIDDEN, hidden)
            }
            runCatching { appContext.startService(intent) }
        }

        fun refreshAppearance(context: Context) {
            val running = activeInstance
            if (running != null) {
                mainHandler.post { running.applyThemeStyling() }
                return
            }
            val appContext = context.applicationContext
            val prefs = BotHubPrefs(appContext)
            if (!prefs.floatingBallEnabled || !canDrawOverlays(appContext)) return
            val intent = Intent(appContext, FloatingCaptureService::class.java).apply {
                action = ACTION_REFRESH_APPEARANCE
            }
            runCatching { appContext.startService(intent) }
        }
    }
}
