import type { Machine } from './guidanceModel';
import type { ConsumeReceipt, ReadyMetadata } from './nativeCaptureAdapter';

export type CaptureState = {
  generation: number | null;
  frameId: string | null;
  status: 'IDLE' | 'ARMED' | 'ENCODING' | 'READY' | 'CONSUMING' | 'CONSUMED' | 'CANCELLED' | 'ERROR';
  triggerCount: number;
  result: ReadyMetadata | null;
};

// Native frame IDs are unique, increasing decimal Longs across generations.
// Keep one high-water mark, not an ever-growing collection of face/frame data.
export class CaptureController {
  private highestFrameId = '0';
  state: CaptureState = { generation: null, frameId: null, status: 'IDLE', triggerCount: 0, result: null };

  arm(generation: number): void {
    this.state = { generation, frameId: null, status: 'ARMED', triggerCount: 0, result: null };
  }

  claimFrame(frameId: string): boolean {
    if (!/^[1-9][0-9]{0,18}$/.test(frameId)
      || (frameId.length === 19 && frameId > '9223372036854775807')) return false;
    if (frameId.length < this.highestFrameId.length
      || (frameId.length === this.highestFrameId.length && frameId <= this.highestFrameId)) return false;
    this.highestFrameId = frameId;
    return true;
  }

  approve(machine: Machine, frameId: string): boolean {
    if (machine.source !== 'live' || machine.phase !== 'SIMULATED_CAPTURE'
      || machine.generation !== this.state.generation || this.state.status !== 'ARMED'
      || frameId !== this.highestFrameId) return false;
    this.state = { ...this.state, frameId, status: 'ENCODING', triggerCount: 1, result: null };
    return true;
  }

  ready(result: ReadyMetadata | null, foreground: boolean): boolean {
    if (!foreground || !result || this.state.status !== 'ENCODING'
      || result.generation !== this.state.generation || result.frameId !== this.state.frameId
      || result.status !== 'READY' || result.exif !== false
      || !Number.isInteger(result.jpegBytes) || result.jpegBytes <= 0 || result.jpegBytes > 4194304
      || !Number.isInteger(result.width) || !Number.isInteger(result.height)
      || result.width <= 0 || result.height <= 0 || result.width * result.height > 2073600
      || ![result.encodeMs, result.selectedAgeMs, result.peakOwnedBytes, result.expiresAtMs].every((n) => Number.isFinite(n) && n >= 0)
      || result.selectedAgeMs > 400 || result.peakOwnedBytes > 16777216 || result.expiresAtMs <= 0) return false;
    this.state = { ...this.state, status: 'READY', result };
    return true;
  }

  consuming(generation: number, frameId: string): boolean {
    if (this.state.status !== 'READY' || this.state.generation !== generation || this.state.frameId !== frameId) return false;
    this.state = { ...this.state, status: 'CONSUMING' };
    return true;
  }

  complete(receipt: ConsumeReceipt, foreground: boolean): boolean {
    const result = this.state.result;
    if (!foreground || this.state.status !== 'CONSUMING' || !result
      || receipt.status !== 'CONSUMED' || receipt.consumed !== true || receipt.exif !== false
      || receipt.generation !== this.state.generation || receipt.frameId !== this.state.frameId
      || receipt.width !== result.width || receipt.height !== result.height || receipt.jpegBytes !== result.jpegBytes) return false;
    this.state = { ...this.state, status: 'CONSUMED' };
    return true;
  }

  fail(generation: number): void {
    if (this.state.generation !== generation) return;
    this.state = { ...this.state, status: 'ERROR', result: null };
  }

  invalidate(): void {
    this.state = { ...this.state, generation: null, frameId: null, status: 'CANCELLED', result: null };
  }

  clear(): void {
    this.state = { generation: null, frameId: null, status: 'IDLE', triggerCount: 0, result: null };
  }
}
