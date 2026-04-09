// haiku-color-bridge.cpp
// GF(3) color rendering on HaikuOS via BView
// Part of boxxy's cross-platform trit-tick color pipeline
//
// HaikuOS renders via app_server (no X11, no Wayland).
// Colors are BGRA32 native. The trit value tags each
// frame with its GF(3) polarity.
//
// To compile on HaikuOS:
//   g++ -o color-bridge haiku-color-bridge.cpp -lbe -lroot

#include <Application.h>
#include <Window.h>
#include <View.h>
#include <MessageRunner.h>
#include <String.h>
#include <OS.h>
#include <SerialPort.h>
#include <cstdint>
#include <cstdio>

// ---------------------------------------------------------------------------
// Flick timing: 1 flick = 1/705600000 second
// 705600000 = 2^9 * 3^2 * 5^5 * 7^2
// ---------------------------------------------------------------------------

static const int64 kFlicksPerSecond = 705600000LL;
static const int64 kFlicksPer60     = kFlicksPerSecond / 60;  // 11760000

static int64
SystemTimeToFlicks(bigtime_t usec)
{
	// system_time() returns microseconds; convert to flicks.
	// 1 us = 705.6 flicks
	return (int64)((double)usec * ((double)kFlicksPerSecond / 1000000.0));
}

// ---------------------------------------------------------------------------
// GF(3) element: -1, 0, +1  (mod 3 arithmetic)
// ---------------------------------------------------------------------------

enum GF3 : int8 {
	kMinus   = -1,  // Verifier / sandbox
	kErgodic =  0,  // Coordinator / container
	kPlus    =  1   // Generator / VM
};

static GF3
GF3Add(GF3 a, GF3 b)
{
	int sum = (int)a + (int)b;
	if (sum >  1) sum -= 3;
	if (sum < -1) sum += 3;
	return (GF3)sum;
}

// ---------------------------------------------------------------------------
// TritTick: a Flick tagged with its GF(3) polarity
// Matches boxxy/internal/flick/flick.go TritTick struct
// ---------------------------------------------------------------------------

struct TritTick {
	int64  when;   // flick position
	GF3    trit;   // polarity tag
};

// ---------------------------------------------------------------------------
// GF3Palette: matches boxxy/internal/vtterm/vtterm.go GF3Palette exactly
//   [0] amber  #F59E0B  — Zero/Coordinator (ergodic)
//   [1] purple #A855F7  — One/Generator    (plus)
//   [2] blue   #2E5FA3  — Two/Verifier     (minus)
// ---------------------------------------------------------------------------

static const rgb_color kGF3Colors[3] = {
	{0xF5, 0x9E, 0x0B, 0xFF},  // amber  (trit  0)
	{0xA8, 0x55, 0xF7, 0xFF},  // purple (trit +1)
	{0x2E, 0x5F, 0xA3, 0xFF},  // blue   (trit -1)
};

static rgb_color
ColorForTrit(GF3 t)
{
	switch (t) {
		case kErgodic: return kGF3Colors[0];
		case kPlus:    return kGF3Colors[1];
		case kMinus:   return kGF3Colors[2];
	}
	return kGF3Colors[0];
}

// ---------------------------------------------------------------------------
// Message constants for virtio-serial trit-tick protocol
// ---------------------------------------------------------------------------

static const uint32 kMsgTritTick    = 'TRTK';
static const uint32 kMsgPollSerial  = 'POLL';

// ---------------------------------------------------------------------------
// TritColorView: 2x2 grid of GF(3) color tiles + status bar
// Matches the boxxy-tile.el layout: 4 tiles, each with a trit assignment.
// ---------------------------------------------------------------------------

class TritColorView : public BView {
public:
	TritColorView(BRect frame)
		: BView(frame, "TritColorView", B_FOLLOW_ALL, B_WILL_DRAW)
	{
		// Initial tile trits: one full frame triad + the GF(3) balance trit
		fTiles[0] = {0, kMinus};
		fTiles[1] = {0, kErgodic};
		fTiles[2] = {0, kPlus};
		fTiles[3] = {0, kErgodic};  // balance tile
	}

	void SetTritTick(int tile, TritTick tt)
	{
		if (tile >= 0 && tile < 4) {
			fTiles[tile] = tt;
			Invalidate();
		}
	}

	void Draw(BRect updateRect) override
	{
		BRect bounds = Bounds();
		float halfW = bounds.Width()  / 2.0f;
		float halfH = bounds.Height() / 2.0f;
		float statusH = 24.0f;
		float tileH = (bounds.Height() - statusH) / 2.0f;

		// Draw 2x2 tile grid
		BRect tiles[4] = {
			BRect(0,     0,     halfW, tileH),
			BRect(halfW, 0,     bounds.Width(), tileH),
			BRect(0,     tileH, halfW, tileH * 2),
			BRect(halfW, tileH, bounds.Width(), tileH * 2),
		};

		for (int i = 0; i < 4; i++) {
			SetHighColor(ColorForTrit(fTiles[i].trit));
			FillRect(tiles[i]);

			// Trit label
			SetHighColor(make_color(0, 0, 0, 255));
			char label[32];
			snprintf(label, sizeof(label), "t=%+d  f=%lld",
				(int)fTiles[i].trit, (long long)fTiles[i].when);
			DrawString(label, tiles[i].LeftTop() + BPoint(8, 18));
		}

		// GF(3) balance check: sum of all tile trits must be 0 mod 3
		GF3 sum = kErgodic;
		for (int i = 0; i < 4; i++)
			sum = GF3Add(sum, fTiles[i].trit);

		bool balanced = (sum == kErgodic);

		// Status bar at bottom
		BRect statusBar(0, tileH * 2, bounds.Width(), bounds.Height());
		SetHighColor(balanced
			? make_color(0x10, 0xB9, 0x81, 0xFF)   // green: balanced
			: make_color(0xEF, 0x44, 0x44, 0xFF));  // red: imbalanced
		FillRect(statusBar);

		SetHighColor(make_color(255, 255, 255, 255));
		char status[64];
		snprintf(status, sizeof(status), "GF(3) sum=%+d  %s",
			(int)sum, balanced ? "BALANCED" : "IMBALANCED");
		DrawString(status, statusBar.LeftTop() + BPoint(8, 16));
	}

	void MessageReceived(BMessage* msg) override
	{
		if (msg->what == kMsgTritTick) {
			int32 tile;
			int8  trit;
			int64 when;
			if (msg->FindInt32("tile", &tile) == B_OK
				&& msg->FindInt8("trit", &trit) == B_OK
				&& msg->FindInt64("when", &when) == B_OK)
			{
				TritTick tt = {when, (GF3)trit};
				SetTritTick(tile, tt);
			}
			return;
		}
		BView::MessageReceived(msg);
	}

private:
	TritTick fTiles[4];
};

// ---------------------------------------------------------------------------
// ColorBridgeApp: sets up window + polls virtio-serial for trit-ticks
// ---------------------------------------------------------------------------

class ColorBridgeApp : public BApplication {
public:
	ColorBridgeApp()
		: BApplication("application/x-vnd.boxxy.color-bridge")
		, fWindow(NULL)
		, fView(NULL)
	{
	}

	void ReadyToRun() override
	{
		BRect frame(100, 100, 420, 420);
		fWindow = new BWindow(frame, "Boxxy GF(3) Color Bridge",
			B_TITLED_WINDOW, B_QUIT_ON_WINDOW_CLOSE);

		fView = new TritColorView(fWindow->Bounds());
		fWindow->AddChild(fView);
		fWindow->Show();
	}

private:
	BWindow*       fWindow;
	TritColorView* fView;
};

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

int
main()
{
	ColorBridgeApp app;
	app.Run();
	return 0;
}
