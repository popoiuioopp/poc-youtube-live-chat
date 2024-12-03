package youtube

import "fmt"

// fetchLiveVideoID fetches the live video ID from a given channel ID
func FetchLiveVideoID(channelID string) (string, error) {
	if YouTubeService == nil {
		return "", fmt.Errorf("YouTube service not initialized")
	}

	call := YouTubeService.Search.List([]string{"id"}).
		ChannelId(channelID).
		EventType("live").
		Type("video").
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("error fetching live video id: %v", err)
	}

	if len(response.Items) == 0 {
		return "", nil // No live video found
	}

	videoID := response.Items[0].Id.VideoId
	fmt.Printf("FOUND live stream for %s : %s", channelID, videoID)
	return videoID, nil
}

// fetchLiveChatIDByVideoID retrieves the chat ID of a live stream given a video ID
func FetchLiveChatIDByVideoID(videoID string) (string, error) {
	if YouTubeService == nil {
		return "", fmt.Errorf("YouTube service not initialized")
	}

	call := YouTubeService.Videos.List([]string{"liveStreamingDetails"}).Id(videoID)
	response, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("error retrieving video details: %v", err)
	}

	if len(response.Items) == 0 {
		return "", fmt.Errorf("no video found with id: %s", videoID)
	}

	liveStreamingDetails := response.Items[0].LiveStreamingDetails
	if liveStreamingDetails == nil || liveStreamingDetails.ActiveLiveChatId == "" {
		return "", fmt.Errorf("live chat not available for this video")
	}

	return liveStreamingDetails.ActiveLiveChatId, nil
}
