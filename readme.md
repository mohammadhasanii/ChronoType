# ChronoType ⌨️⏱️

**GitHub Repository:** [https://github.com/mohammadhasanii/ChronoType](https://github.com/mohammadhasanii/ChronoType)

![ChronoType Showcase](./go-type.jpg)

ChronoType is a Windows desktop application that monitors your keyboard activity, providing daily statistics and visualizations of your typing patterns through a local web interface.

## 🚀 Features

* Real-time keystroke counting (updates via periodic polling).
* Daily tracking of total keystrokes, average keystrokes per minute, and active typing minutes.
* Persistent storage of daily data in a JSON file (`keystroke_data.json`).
* Web-based dashboard to view statistics.
* Interactive charts for visualizing daily keystroke totals and typing speed (avg/min).
* Detailed table view of historical daily data.
* Dark mode support for the web interface.
* Utilizes a low-level Windows keyboard hook for system-wide tracking.

## 📋 Prerequisites

* Windows Operating System (required for Windows API calls and the pre-built executable).
* **For running from source:**
    * Go (version 1.18 or later is recommended).
    * Git (for cloning the repository).

## 🛠️ Installation & Setup

You can run ChronoType either from its source code or by using the pre-built executable for Windows.

### Option 1: Running from Source

1.  **Ensure Prerequisites:** Make sure you have Go and Git installed on your Windows system.
2.  **Clone the Repository:** Open a terminal or Command Prompt and run:
    ```bash
    git clone [https://github.com/mohammadhasanii/ChronoType.git](https://github.com/mohammadhasanii/ChronoType.git)
    ```
3.  **Navigate to Project Directory:**
    ```bash
    cd ChronoType
    ```
    (Note: The main branch is typically `master` or `main`.)
4.  **Run the Application:**
    ```bash
    go run app.go
    ```
    The application will compile and start.

### Option 2: Using the Pre-built Executable (Windows)

1.  **Download:** Navigate to the [Releases page](https://github.com/mohammadhasanii/ChronoType/releases) of the ChronoType GitHub repository. (If a releases page is not available, the `.exe` might be located elsewhere in the repository, or you may need to build it yourself if not provided directly).
2.  Download the latest `.exe` file for Windows (e.g., `ChronoType.exe`).
3.  **Save:** Place the downloaded `ChronoType.exe` file in a directory of your choice on your computer (e.g., `C:\Program Files\ChronoType` or `D:\Tools\ChronoType`).

## ⌨️ Usage

* **If running from source (using `go run app.go`):**
    * After executing the command, the application will start monitoring keystrokes.
    * A console window will appear, displaying logs and status messages. **Keep this window open** as long as you want ChronoType to track your activity. Closing it will stop the application.

* **If using the pre-built executable (`ChronoType.exe`):**
    1.  Navigate to the directory where you saved `ChronoType.exe`.
    2.  Double-click `ChronoType.exe` to run it.
    3.  A console window will likely open, showing that the application is running and the keyboard hook is active. **This window must remain open** for ChronoType to function. Closing the console window will terminate the application.

**Once the application is running (via either method):**

1.  ChronoType will immediately start monitoring your keystrokes system-wide.
2.  Open your preferred web browser (e.g., Chrome, Firefox, Edge).
3.  Navigate to the following address: `http://localhost:8080`
4.  The ChronoType dashboard will load, displaying your typing statistics. The data on this page will update periodically.
5.  Your keystroke data is automatically saved to a file named `keystroke_data.json`.
    * If running from source, this file will typically be created in the root of the cloned `ChronoType` project directory.
    * If running the `.exe` file, `keystroke_data.json` will be created in the same directory where `ChronoType.exe` is located and executed from.

## 🛑 Stopping the Application

* **If running from source (via `go run`):** Go to the terminal window where the application is running and press `Ctrl+C`. Then, you can close the terminal window.
* **If running the `.exe`:** Simply close the console window that opened when you launched `ChronoType.exe`.

## 💻 Technologies Used

* **Backend:** Go (Golang)
* **Frontend:** HTML, CSS (Tailwind CSS via CDN), JavaScript (with Chart.js via CDN for charts)
* **Keyboard Monitoring:** Windows API (via Go's `syscall` package)

## ⚠️ Important Note on Permissions & Security

This application uses a low-level keyboard hook to monitor keystrokes system-wide. This functionality may require appropriate permissions or could be flagged by some antivirus or security software as potentially intrusive due to its nature. Please use this application responsibly and ensure you understand its behavior.

---