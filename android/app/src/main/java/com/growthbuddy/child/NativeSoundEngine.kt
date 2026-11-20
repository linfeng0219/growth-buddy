package com.growthbuddy.child

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioManager
import android.media.SoundPool
import android.os.Bundle
import android.os.SystemClock
import android.speech.tts.TextToSpeech
import java.io.File
import java.io.FileOutputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.util.Locale
import java.util.concurrent.Executors
import kotlin.math.PI
import kotlin.math.exp
import kotlin.math.sin

/**
 * 原生低延迟、高响度音效与语音引擎：
 * - 使用 SoundPool（USAGE_GAME）预加载内存中合成的 16-bit PCM WAV，播放延迟 <20ms，满幅归一化确保扬声器洪亮清晰。
 * - 语音朗读使用 STREAM_MUSIC 满音量与较利落语速，避免发声迟滞。
 */
class NativeSoundEngine(context: Context) {

    private val appContext = context.applicationContext
    private val ioExecutor = Executors.newSingleThreadExecutor()
    private val audioManager = appContext.getSystemService(Context.AUDIO_SERVICE) as AudioManager

    private val soundPool: SoundPool
    @Volatile private var muted: Boolean = false

    @Volatile private var sfxJumpTick = 0
    @Volatile private var sfxCountdown = 0
    @Volatile private var sfxStart = 0
    @Volatile private var sfxFeed = 0
    @Volatile private var sfxPat = 0
    @Volatile private var sfxWin = 0
    @Volatile private var sfxError = 0

    private var tts: TextToSpeech? = null
    @Volatile private var ttsReady: Boolean = false

    init {
        val attrs = AudioAttributes.Builder()
            .setUsage(AudioAttributes.USAGE_GAME)
            .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
            .build()
        soundPool = SoundPool.Builder()
            .setMaxStreams(6)
            .setAudioAttributes(attrs)
            .build()

        ioExecutor.execute { prepareAllWavEffects() }

        tts = TextToSpeech(appContext) { status ->
            if (status == TextToSpeech.SUCCESS) {
                var res = tts?.setLanguage(Locale.SIMPLIFIED_CHINESE)
                    ?: TextToSpeech.LANG_NOT_SUPPORTED
                if (res == TextToSpeech.LANG_MISSING_DATA || res == TextToSpeech.LANG_NOT_SUPPORTED) {
                    res = tts?.setLanguage(Locale.CHINESE) ?: TextToSpeech.LANG_NOT_SUPPORTED
                }
                if (res == TextToSpeech.LANG_MISSING_DATA || res == TextToSpeech.LANG_NOT_SUPPORTED) {
                    tts?.setLanguage(Locale.getDefault())
                }
                ttsReady = true
                tts?.setAudioAttributes(
                    AudioAttributes.Builder()
                        .setUsage(AudioAttributes.USAGE_GAME)
                        .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                        .build()
                )
                tts?.setPitch(1.16f)
                tts?.setSpeechRate(1.18f)
            }
        }
    }

    fun setMuted(isMuted: Boolean) {
        muted = isMuted
        if (isMuted) tts?.stop()
    }

    fun isMuted(): Boolean = muted

    fun isMediaVolumeLow(): Boolean {
        val cur = audioManager.getStreamVolume(AudioManager.STREAM_MUSIC)
        val max = audioManager.getStreamMaxVolume(AudioManager.STREAM_MUSIC).coerceAtLeast(1)
        return cur.toFloat() / max < 0.45f
    }

    fun boostMediaVolumeToRecommended() {
        val max = audioManager.getStreamMaxVolume(AudioManager.STREAM_MUSIC).coerceAtLeast(1)
        val target = (max * 0.80f).toInt().coerceAtLeast(1)
        val cur = audioManager.getStreamVolume(AudioManager.STREAM_MUSIC)
        if (cur < target) {
            audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, target, AudioManager.FLAG_SHOW_UI)
        }
    }

    fun playJumpTick() = playId(sfxJumpTick, 1.0f)
    fun playCountdown() = playId(sfxCountdown, 1.0f)
    fun playStart() = playId(sfxStart, 1.0f)
    fun playFeed() = playId(sfxFeed, 1.0f)
    fun playPat() = playId(sfxPat, 0.95f)
    fun playWin() = playId(sfxWin, 1.0f)
    fun playError() = playId(sfxError, 0.90f)

    fun speak(text: String, flush: Boolean = true): Boolean {
        if (muted || text.isBlank() || !ttsReady) return false
        val params = Bundle().apply {
            putFloat(TextToSpeech.Engine.KEY_PARAM_VOLUME, 1.0f)
            putInt(TextToSpeech.Engine.KEY_PARAM_STREAM, AudioManager.STREAM_MUSIC)
        }
        val mode = if (flush) TextToSpeech.QUEUE_FLUSH else TextToSpeech.QUEUE_ADD
        val res = tts?.speak(text, mode, params, "gb_${SystemClock.elapsedRealtime()}")
        return res == TextToSpeech.SUCCESS
    }

    private fun playId(soundId: Int, volume: Float) {
        if (muted || soundId == 0) return
        soundPool.play(soundId, volume, volume, 1, 0, 1.0f)
    }

    private fun prepareAllWavEffects() {
        val dir = File(appContext.cacheDir, "gb_sfx_v2").apply { mkdirs() }

        fun loadOrGen(name: String, generator: () -> ShortArray): Int {
            val file = File(dir, "$name.wav")
            if (!file.exists() || file.length() < 64) {
                writeWav16Mono(file, generator(), SAMPLE_RATE)
            }
            return soundPool.load(file.absolutePath, 1)
        }

        // 1. 跳绳单次计数清脆击打叮咚（55ms，响亮利落无拖尾）
        sfxJumpTick = loadOrGen("jump_tick") {
            synthToneSequence(
                listOf(
                    ToneNote(1174.66, 0.025, 0.95, 1.0),
                    ToneNote(1760.00, 0.045, 0.85, 0.6)
                ),
                overlapSec = 0.015
            )
        }

        // 2. 倒计时 3-2-1 提示音
        sfxCountdown = loadOrGen("countdown") {
            synthToneSequence(listOf(ToneNote(987.77, 0.11, 0.95, 0.45)))
        }

        // 3. 开始号角（上扬三音）
        sfxStart = loadOrGen("start") {
            synthToneSequence(
                listOf(
                    ToneNote(783.99, 0.075, 0.90, 0.4),
                    ToneNote(1046.50, 0.075, 0.92, 0.4),
                    ToneNote(1567.98, 0.16, 0.96, 0.5)
                ),
                overlapSec = 0.01
            )
        }

        // 4. 喂食打卡成功双音
        sfxFeed = loadOrGen("feed") {
            synthToneSequence(
                listOf(
                    ToneNote(659.25, 0.085, 0.92, 0.45),
                    ToneNote(880.00, 0.095, 0.94, 0.45),
                    ToneNote(1318.51, 0.14, 0.90, 0.55)
                ),
                overlapSec = 0.015
            )
        }

        // 5. 摸头 Q 弹音
        sfxPat = loadOrGen("pat") {
            synthToneSequence(
                listOf(
                    ToneNote(587.33, 0.07, 0.88, 0.5),
                    ToneNote(880.00, 0.09, 0.90, 0.55)
                ),
                overlapSec = 0.015
            )
        }

        // 6. 达标 / 升级 / 开蛋欢呼和弦
        sfxWin = loadOrGen("win") {
            synthToneSequence(
                listOf(
                    ToneNote(523.25, 0.09, 0.90, 0.4),
                    ToneNote(659.25, 0.09, 0.92, 0.4),
                    ToneNote(783.99, 0.09, 0.94, 0.4),
                    ToneNote(1046.50, 0.24, 0.96, 0.5)
                ),
                overlapSec = 0.015
            )
        }

        // 7. 温和错误提示音
        sfxError = loadOrGen("error") {
            synthToneSequence(
                listOf(
                    ToneNote(311.13, 0.08, 0.80, 0.4),
                    ToneNote(233.08, 0.12, 0.80, 0.4)
                ),
                overlapSec = 0.01
            )
        }
    }

    fun release() {
        soundPool.release()
        tts?.stop()
        tts?.shutdown()
        ioExecutor.shutdownNow()
    }

    private data class ToneNote(
        val freqHz: Double,
        val durSec: Double,
        val gain: Double,
        val decayRate: Double
    )

    private fun synthToneSequence(notes: List<ToneNote>, overlapSec: Double = 0.0): ShortArray {
        val totalSec = notes.sumOf { it.durSec } - overlapSec * (notes.size - 1).coerceAtLeast(0)
        val totalSamples = (totalSec * SAMPLE_RATE).toInt().coerceAtLeast(1)
        val mix = DoubleArray(totalSamples)
        var cursorSec = 0.0

        for (note in notes) {
            val startIdx = (cursorSec * SAMPLE_RATE).toInt().coerceAtLeast(0)
            val len = (note.durSec * SAMPLE_RATE).toInt().coerceAtLeast(1)
            val attackSamples = (0.004 * SAMPLE_RATE).toInt().coerceAtLeast(1)
            for (i in 0 until len) {
                val idx = startIdx + i
                if (idx >= totalSamples) break
                val t = i.toDouble() / SAMPLE_RATE
                val progress = i.toDouble() / len
                val attack = if (i < attackSamples) i.toDouble() / attackSamples else 1.0
                val release = if (progress > 0.78) ((1.0 - progress) / 0.22).coerceAtLeast(0.0) else 1.0
                val env = attack * release * exp(-note.decayRate * 4.5 * progress)
                // 基频 + 2次/3次明亮谐波，手机平板外放更响亮透亮
                val wave = 0.68 * sin(2.0 * PI * note.freqHz * t) +
                    0.22 * sin(2.0 * PI * note.freqHz * 2.0 * t) +
                    0.10 * sin(2.0 * PI * note.freqHz * 3.0 * t)
                mix[idx] += wave * env * note.gain
            }
            cursorSec += (note.durSec - overlapSec).coerceAtLeast(0.01)
        }

        // 峰值归一化到 0.94，保证音量饱满无削波
        var peak = 0.0
        for (v in mix) {
            val a = kotlin.math.abs(v)
            if (a > peak) peak = a
        }
        val norm = if (peak > 1e-6) 0.94 / peak else 1.0
        return ShortArray(totalSamples) { i ->
            (mix[i] * norm * Short.MAX_VALUE).toInt().coerceIn(Short.MIN_VALUE.toInt(), Short.MAX_VALUE.toInt()).toShort()
        }
    }

    private fun writeWav16Mono(file: File, pcm: ShortArray, sampleRate: Int) {
        val dataBytes = pcm.size * 2
        val header = ByteBuffer.allocate(44).order(ByteOrder.LITTLE_ENDIAN).apply {
            put("RIFF".toByteArray(Charsets.US_ASCII))
            putInt(36 + dataBytes)
            put("WAVE".toByteArray(Charsets.US_ASCII))
            put("fmt ".toByteArray(Charsets.US_ASCII))
            putInt(16) // PCM chunk size
            putShort(1) // PCM format
            putShort(1) // Mono
            putInt(sampleRate)
            putInt(sampleRate * 2)
            putShort(2) // Block align
            putShort(16) // Bits per sample
            put("data".toByteArray(Charsets.US_ASCII))
            putInt(dataBytes)
        }
        val body = ByteBuffer.allocate(dataBytes).order(ByteOrder.LITTLE_ENDIAN)
        for (s in pcm) body.putShort(s)
        FileOutputStream(file).use { out ->
            out.write(header.array())
            out.write(body.array())
        }
    }

    companion object {
        private const val SAMPLE_RATE = 44100
    }
}
