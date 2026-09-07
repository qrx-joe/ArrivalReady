/**
 * 组装 demo 视频：TTS 配音 → 每场景对齐 → 拼接 → 烧字幕 → 输出。
 *
 * 输入：scripts/demo/video/scenes.json + raw/*.webm（record_*.mjs 产物）
 * 输出：docs/demo/arrivalready_demo.mp4 与同名 .srt
 *
 * 依赖：ffmpeg/ffprobe 在 PATH；edge-tts（pip）在 PATH。
 */
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const RAW = path.join(HERE, "raw");
const WORK = path.join(HERE, "work");
const OUT = path.resolve(HERE, "../../../docs/demo");
fs.mkdirSync(WORK, { recursive: true });
fs.mkdirSync(OUT, { recursive: true });

const cfg = JSON.parse(fs.readFileSync(path.join(HERE, "scenes.json"), "utf8"));
const { width: W, height: H, fps } = cfg.video;
const NARR_DELAY_MS = 800; // 每场景开场留白
const TAIL_PAD_S = 0.9; // 旁白结束后的呼吸

const sh = (cmd, args, opts = {}) => {
  console.log("  $", cmd, args.slice(0, 6).join(" "), args.length > 6 ? "…" : "");
  execFileSync(cmd, args, { stdio: ["ignore", "ignore", "inherit"], cwd: WORK, ...opts });
};

const dur = (file) => {
  const out = execFileSync(
    "ffprobe",
    ["-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", file],
  ).toString().trim();
  return parseFloat(out);
};

function srtTime(t) {
  const h = String(Math.floor(t / 3600)).padStart(2, "0");
  const m = String(Math.floor((t % 3600) / 60)).padStart(2, "0");
  const s = String(Math.floor(t % 60)).padStart(2, "0");
  const ms = String(Math.round((t - Math.floor(t)) * 1000)).padStart(3, "0");
  return `${h}:${m}:${s},${ms}`;
}

async function main() {
  // ---------- 1. TTS（edge-tts 偶发 NoAudioReceived，重试兜底） ----------
  const ttsDur = (file) => {
    try {
      const d = dur(file);
      return Number.isFinite(d) ? d : 0;
    } catch {
      return 0;
    }
  };
  const scenes = [];
  for (const sc of cfg.scenes) {
    const mp3 = path.join(RAW, `tts_${sc.id}.mp3`);
    if (fs.existsSync(mp3) && ttsDur(mp3) <= 1) fs.unlinkSync(mp3); // 无效残留
    if (!fs.existsSync(mp3) || process.env.FORCE_TTS) {
      let ok = false;
      for (let attempt = 1; attempt <= 5 && !ok; attempt++) {
        try {
          sh("edge-tts", [
            "--voice", cfg.voice.voice,
            "--rate", cfg.voice.rate,
            "--text", sc.narration,
            "--write-media", mp3,
          ]);
          if (fs.existsSync(mp3) && ttsDur(mp3) > 1) ok = true;
        } catch {
          console.log(`    tts retry #${attempt} for ${sc.id}`);
          await new Promise((r) => setTimeout(r, 1500 * attempt));
        }
      }
      if (!ok) throw new Error(`TTS 连续失败: ${sc.id}`);
    }
    scenes.push({ ...sc, mp3, narrDur: dur(mp3) });
  }

  // ---------- 2. 每场景：视频对齐 + 配音混入 ----------
  let cursor = 0;
  const srtLines = [];
  for (const sc of scenes) {
    const src = sc.card
      ? path.join(RAW, { open: "card_open.webm", engineering: "card_engineering.webm", close: "card_close.webm" }[sc.card])
      : path.join(RAW, sc.src);
    if (!fs.existsSync(src)) throw new Error(`缺片段: ${src}`);
    const durS = Math.max(sc.min_s, NARR_DELAY_MS / 1000 + sc.narrDur + TAIL_PAD_S);
    sc.dur = durS;

    const part = path.join(WORK, `${sc.id}.mp4`);
    sh("ffmpeg", [
      "-y", "-v", "error",
      "-i", src, "-i", sc.mp3,
      "-filter_complex",
      `[0:v]scale=${W}:${H}:force_original_aspect_ratio=decrease,` +
        `pad=${W}:${H}:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=${fps},` +
        `tpad=stop_mode=clone:stop_duration=600,` +
        `fade=t=in:st=0:d=0.3,fade=t=out:st=${(durS - 0.4).toFixed(2)}:d=0.4[v];` +
        `[1:a]adelay=${NARR_DELAY_MS}|${NARR_DELAY_MS},apad[a]`,
      "-map", "[v]", "-map", "[a]",
      "-t", durS.toFixed(3),
      "-c:v", "libx264", "-preset", "medium", "-crf", "20",
      "-c:a", "aac", "-b:a", "160k", "-ar", "44100", "-ac", "2",
      "-pix_fmt", "yuv420p",
      part,
    ]);

    // 字幕：按字符数比例铺满旁白窗口
    const chars = sc.subs.reduce((a, s) => a + s.length, 0);
    let t0 = cursor + NARR_DELAY_MS / 1000;
    for (const line of sc.subs) {
      const share = (line.length / chars) * sc.narrDur;
      srtLines.push(
        `${srtLines.length + 1}\n${srtTime(t0)} --> ${srtTime(t0 + share - 0.05)}\n${line}\n`,
      );
      t0 += share;
    }
    cursor += durS;
    console.log(`  ✔ ${sc.id} ${durS.toFixed(1)}s`);
  }

  // ---------- 3. 拼接 ----------
  const total = cursor;
  const concatIn = path.join(WORK, "parts.txt");
  fs.writeFileSync(concatIn, scenes.map((s) => `file '${path.join(WORK, `${s.id}.mp4`)}'`.replaceAll("\\", "/")).join("\n"));
  const silent = path.join(WORK, "concat.mp4");
  sh("ffmpeg", ["-y", "-v", "error", "-f", "concat", "-safe", "0", "-i", concatIn, "-c", "copy", silent]);

  // ---------- 4. 字幕烧录 + 输出 ----------
  // 相对路径 + cwd 规避 Windows 盘符冒号在 filtergraph 里的多层转义；
  // 字体由 fontconfig 按系统字体名解析（Microsoft YaHei 随 Windows 提供）。
  const srt = path.join(OUT, "arrivalready_demo.srt");
  fs.writeFileSync(srt, "\ufeff" + srtLines.join("\n"), "utf8");
  fs.copyFileSync(srt, path.join(WORK, "subs.srt"));
  const finalMp4 = path.join(OUT, "arrivalready_demo.mp4");
  sh("ffmpeg", [
    "-y", "-v", "error",
    "-i", silent,
    "-vf",
    "subtitles=subs.srt:force_style='FontName=Microsoft YaHei,FontSize=14,PrimaryColour=&H00FFFFFF,OutlineColour=&HB4000000,BorderStyle=1,Outline=1.3,Shadow=0.7,MarginV=44,Spacing=0.3'",
    "-c:v", "libx264", "-preset", "medium", "-crf", "20",
    "-c:a", "copy", "-pix_fmt", "yuv420p",
    "-movflags", "+faststart",
    finalMp4,
  ]);

  sh("ffmpeg", [
    "-y", "-v", "error",
    "-i", finalMp4,
    "-c:v", "copy",
    "-af", "loudnorm=I=-16:TP=-1.5:LRA=11",
    "-c:a", "aac", "-b:a", "160k",
    "-movflags", "+faststart",
    finalMp4.replace(".mp4", ".loud.mp4"),
  ]);
  fs.renameSync(finalMp4.replace(".mp4", ".loud.mp4"), finalMp4);

  console.log(`\n总时长 ${total.toFixed(1)}s → ${finalMp4}`);
  console.log(`字幕 ${srt}`);
}

main().catch((e) => {
  console.error("❌", e.message);
  process.exit(1);
});
