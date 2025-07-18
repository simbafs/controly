import { Html5QrcodeScanner } from 'html5-qrcode';

export function setupQrCodeScanner(element: HTMLDivElement, onScanSuccess: (decodedText: string) => void) {
  const qrCodeSuccessCallback = (decodedText: string, decodedResult: any) => {
    // Handle the scanned QR code text
    console.log(`QR Code matched = ${decodedText}`, decodedResult);
    onScanSuccess(decodedText);
    // Optionally, stop the scanner after a successful scan
    html5QrcodeScanner.clear();
  };

  const html5QrcodeScanner = new Html5QrcodeScanner(
    element.id,
    { fps: 10, qrbox: { width: 250, height: 250 } },
    /* verbose= */ false
  );

  html5QrcodeScanner.render(qrCodeSuccessCallback, (errorMessage) => {
    // parse error, ideally ignore it.
    console.warn(`QR Code Scan Error: ${errorMessage}`);
  });
}
