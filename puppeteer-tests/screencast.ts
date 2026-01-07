import { Page, CDPSession } from 'puppeteer';
import * as fs from 'fs';
import * as path from 'path';
import { spawnSync } from 'child_process';
import { outputDir } from './paths';

/**
 * Records a video of a Puppeteer page session using Chrome DevTools Protocol (CDP).
 *
 * It captures frames as PNGs and stitches them into an MP4 video using ffmpeg.
 * Useful for debugging failing tests or verifying UI behavior.
 *
 * Usage:
 * ```typescript
 * const recorder = new ScreencastRecorder('screencast');
 * await recorder.start(page);
 * // ... perform actions ...
 * await recorder.stop();
 * ```
 *
 * To record a video, run the test with the following flag:
 * bazelisk test --test_env=RECORD_VIDEO=true //perf/modules/explore-multi-sk/...
 *
 * The resulting mp4 video will be under this path:
 * OUTPUT=$(bazelisk info output_path)
 * $OUTPUT/k8-fastbuild/testlogs/perf/modules/{module_name}/{test_name}/test.outputs/screencast.mp4
 *
 * Example of module_name: explore-multi-sk
 * Example of test_name: explore-multi-sk_puppeteer_test
 */
export class ScreencastRecorder {
  private client: CDPSession | null = null;

  private page: Page | null = null;

  private frameCount = 0;

  private sessionOutputDir: string;

  private prefix: string;

  private fps: number;

  constructor(testName: string, fps = 5) {
    // Sanitize test name for file system
    const safeTestName = testName.replace(/[^a-z0-9]/gi, '_').toLowerCase();
    this.sessionOutputDir = path.join(outputDir(), 'screencasts', safeTestName);
    this.prefix = safeTestName;
    this.fps = fps;
  }

  async start(page: Page) {
    this.page = page;
    if (fs.existsSync(this.sessionOutputDir)) {
      fs.rmSync(this.sessionOutputDir, { recursive: true, force: true });
    }
    fs.mkdirSync(this.sessionOutputDir, { recursive: true });

    await this.enableUrlOverlay(page);

    try {
      this.client = await page.target().createCDPSession();

      this.client.on('Page.screencastFrame', async (frameObject) => {
        try {
          const { data, sessionId } = frameObject;
          const buffer = Buffer.from(data, 'base64');
          // Use sequential numbering for ffmpeg
          const filename = path.join(
            this.sessionOutputDir,
            `${this.prefix}_${this.frameCount.toString().padStart(6, '0')}.png`
          );
          fs.writeFileSync(filename, buffer);
          await this.client?.send('Page.screencastFrameAck', { sessionId });
          this.frameCount++;
        } catch (e) {
          console.error('Error handling screencast frame:', e);
        }
      });

      await this.client.send('Page.startScreencast', {
        format: 'png',
        everyNthFrame: 1,
        quality: 100,
      });
      console.log(`Screencast started. Frames will be saved to ${this.sessionOutputDir}`);
    } catch (e) {
      console.error('Failed to start screencast:', e);
    }
  }

  async stop() {
    // Wait for 3 seconds to capture final state
    await new Promise((resolve) => setTimeout(resolve, 3000));

    if (this.client) {
      try {
        await this.client.send('Page.stopScreencast');
        await this.client.detach();
      } catch (e) {
        console.error('Error stopping screencast:', e);
      } finally {
        this.client = null;
      }
    }

    // Capture explicit final frame to ensure the last state is visible
    if (this.page && !this.page.isClosed()) {
      try {
        const filename = path.join(
          this.sessionOutputDir,
          `${this.prefix}_${this.frameCount.toString().padStart(6, '0')}.png`
        );
        await this.page.screenshot({ path: filename });
        this.frameCount++;
      } catch (e) {
        console.error('Error taking final screenshot:', e);
      }
    }

    console.log(`Screencast stopped. Saved ${this.frameCount} frames.`);
    if (this.frameCount > 0) {
      this.createVideo();
    }
  }

  /**
   * Injects a sticky header to display the current URL.
   */
  private async enableUrlOverlay(page: Page) {
    const injectStyles = async () => {
      await page.evaluate(() => {
        if (document.getElementById('puppeteer-url-overlay')) return;

        const overlay = document.createElement('div');
        overlay.id = 'puppeteer-url-overlay';
        overlay.style.position = 'fixed';

        // Move to top
        overlay.style.top = '0';
        overlay.style.left = '0';
        overlay.style.width = '100%';

        overlay.style.backgroundColor = '#ffeb3b'; // Yellow background
        overlay.style.color = 'black';
        overlay.style.zIndex = '999999';
        overlay.style.fontFamily = 'monospace';
        overlay.style.fontSize = '12px';
        overlay.style.padding = '4px 8px';

        // Border on bottom
        overlay.style.borderBottom = '1px solid #ccc';

        overlay.style.whiteSpace = 'nowrap';
        overlay.style.overflow = 'hidden';
        overlay.style.textOverflow = 'ellipsis';
        overlay.style.opacity = '0.9';
        overlay.textContent = window.location.href;
        document.body.appendChild(overlay);
      });
    };

    // Inject immediately
    await injectStyles();

    // Re-inject/Update on navigation
    page.on('framenavigated', async (frame) => {
      if (frame === page.mainFrame()) {
        await injectStyles();
        await page.evaluate(() => {
          const overlay = document.getElementById('puppeteer-url-overlay');
          if (overlay) overlay.textContent = window.location.href;
        });
      }
    });
  }

  private createVideo() {
    const videoPath = path.join(this.sessionOutputDir, '..', `${this.prefix}.mp4`);
    try {
      // Run ffmpeg
      const args = [
        '-framerate',
        this.fps.toString(),
        '-i',
        path.join(this.sessionOutputDir, `${this.prefix}_%06d.png`),
        '-c:v',
        'libx264',
        '-pix_fmt',
        'yuv420p',
        '-y',
        videoPath,
      ];

      console.log(`Generating video: ffmpeg ${args.join(' ')}`);
      const result = spawnSync('ffmpeg', args);

      if (result.error) {
        console.error('Failed to run ffmpeg:', result.error);
      } else if (result.status !== 0) {
        console.error('ffmpeg failed:', result.stderr.toString());
      } else {
        console.log(`Video saved to ${videoPath}`);
      }
    } catch (e) {
      console.error('Error creating video:', e);
    }
  }
}
