// The Oh My Llama mark, shown as the empty-state placeholder above the composer.
//
// Geometry is the same silhouette as app/assets/omll/llama.svg - keep the two in
// sync. The mark's bounding box is x 25..86, y 2..97 (centre 55.5, 49.5), so it
// is recentred here, and scaled to 0.86 - just under the 0.885 that inscribes a
// 61x95 box in the disc, which is only visible in dark mode but must not clip.
// The box is 68px rather than the stock 60 because this mark is tall and narrow:
// at 60px it looked noticeably slighter than the composer below it.
export default function Logo() {
  return (
    <div className="flex mb-8 justify-center select-none">
      <div className="relative select-none">
        <svg
          width="68"
          height="68"
          viewBox="0 0 100 100"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
          className="select-none"
        >
          <circle cx="50" cy="50" r="50" fill="white" />
          <g transform="translate(50 50) scale(0.86) translate(-55.5 -49.5)">
            <g fill="black">
              {/* ears: tall, narrow, slightly splayed */}
              <path d="M41 27 C36 19 36 7 41 2 C47 8 48 20 47 27 Z" />
              <path d="M58 27 C56 18 59 6 65 2 C69 10 66 21 63 27 Z" />
              {/* neck sweeping up into the head, then out to the muzzle */}
              <path
                d="M25 97 C25 68 27 50 34 39 C39 30 46 24 55 23
                   L75 23 C82 23 86 28 86 35 C86 41 81 45 75 45
                   L62 45 C57 47 55 52 54 60 C53 72 55 85 57 97 Z"
              />
            </g>
            {/* eye, knocked out of the silhouette */}
            <circle className="eye" cx="52" cy="32" r="3.4" fill="white" />
          </g>
        </svg>
        <style>{`
          .eye {
            transform-origin: center;
            transform-box: fill-box;
            animation: blink 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94);
            animation-delay: 2s;
            animation-fill-mode: both;
          }

          @keyframes blink {
            0%, 100% {
              transform: scaleY(1);
            }
            15% {
              transform: scaleY(0.4);
            }
            40% {
              transform: scaleY(0.01);
            }
            65% {
              transform: scaleY(0.4);
            }
          }
        `}</style>
      </div>
    </div>
  );
}
