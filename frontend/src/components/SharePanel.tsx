import { QRCodeSVG } from 'qrcode.react'
import { useToast } from '../contexts/ToastContext'
import { copyText } from '../utils/clipboard'

export function SharePanel({ url }: { url: string }) {
  const toast = useToast()
  const copy = async () => toast((await copyText(url)) ? 'Link copied' : 'Could not copy. Select the link and copy it manually.', 'success')
  const share = async () => {
    if (navigator.share) await navigator.share({ title: 'Vote in my poll', url }).catch(() => undefined)
    else await copy()
  }
  return (
    <div className="share">
      <label htmlFor="share-url" className="label">Public voting link</label>
      <div className="row">
        <input id="share-url" className="input" readOnly value={url} onFocus={(e) => e.currentTarget.select()} />
        <button className="btn" onClick={copy}>Copy</button>
        <button className="btn btn-ghost" onClick={share}>Share</button>
      </div>
      <div className="qr" aria-label="QR code for the voting link"><QRCodeSVG value={url} size={132} marginSize={2} /></div>
    </div>
  )
}
